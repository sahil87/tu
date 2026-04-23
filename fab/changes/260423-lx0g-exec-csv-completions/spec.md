# Spec: Safer process spawning, CSV/Markdown export, and shell completions

**Change**: 260423-lx0g-exec-csv-completions
**Created**: 2026-04-23
**Affected memory**: `docs/memory/cli/data-pipeline.md`, `docs/memory/display/formatting.md`, `docs/memory/sync/multi-machine.md`

## Non-Goals

- Streaming process output via `spawn` to replace `maxBuffer` — deferred to a follow-up change (originally perf #7 from `/fab-discuss`)
- Backwards-compatibility shim for the old `command: string` field on `ToolConfig` — `TOOLS` is internal; no external consumer
- Auto-detection of the user's shell for `tu completions` — explicit argument required (see Design Decisions)
- Dynamic-lookup completion scripts that shell out to `tu` on each tab-press — static generation only
- A `--no-heading` flag for Markdown output — always render `## {title}`; can be added in a follow-up if users request
- Shell support beyond bash/zsh/fish (no PowerShell, nushell, elvish, tcsh)

## CLI: Data Pipeline

### Requirement: TOOLS Registry Shape

The `TOOLS` registry in `src/node/core/fetcher.ts` MUST expose each tool as a `ToolConfig` object with four fields: `name` (display name), `binary` (absolute or on-PATH executable to invoke), `prefixArgs` (array of arguments prepended before runtime arguments, e.g., `[".../index.js"]` for vendor-path invocations), and `needsFilter` (boolean — whether stdout needs `stripNoise` before JSON parsing).

The `ToolConfig` interface in `src/node/core/types.ts` MUST match this shape. No other exported identifiers from `types.ts` change.

#### Scenario: Vendor mode constructs binary + prefixArgs
- **GIVEN** `useVendor` is true (the bundled `vendor/` directory exists next to the running script)
- **WHEN** the `TOOLS` registry is constructed
- **THEN** each entry's `binary` MUST be `"node"`
- **AND** each entry's `prefixArgs` MUST be `[".../${toolName}/index.js"]` resolved under the vendor directory
- **AND** `needsFilter` MUST match the per-tool filter requirement (`false` for `cc`, `true` for `codex` and `oc`)

#### Scenario: Non-vendor mode uses on-PATH binary directly
- **GIVEN** `useVendor` is false (`vendor/` directory does not exist, dev/test mode uses `node_modules/.bin`)
- **WHEN** the `TOOLS` registry is constructed
- **THEN** each entry's `binary` MUST be the resolved path to the `ccusage`/`ccusage-codex`/`ccusage-opencode` launcher in `node_modules/.bin/`
- **AND** each entry's `prefixArgs` MUST be an empty array

### Requirement: Child Process Spawning via execFile

All ccusage child process invocations in `src/node/core/fetcher.ts` MUST use `child_process.execFile` (or the equivalent `execFile` via a promisified wrapper). Direct use of `child_process.exec` is prohibited in this file.

`execFile` SHALL be invoked with the tool's `binary` as the first argument and a single argv array as the second argument, constructed as `[...tool.prefixArgs, period, "--json", ...extraArgs]`. The third argument MUST be the options object carrying `encoding: "utf-8"` and `maxBuffer: 10 * 1024 * 1024` (unchanged from the pre-change value).

Error handling MUST preserve existing behaviour: on process error, warn on stderr with the tool name and error message, resolve the wrapper Promise with an empty string (so downstream parsing produces `EMPTY` totals).

#### Scenario: execFile called with argv array
- **GIVEN** a call to `fetchHistory("cc", "daily", ["--since", "2026-04-01"])` with `useVendor: true`
- **WHEN** the internal spawning wrapper runs
- **THEN** `execFile` MUST be called with `"node"` as the first argument
- **AND** the argv array MUST be `["<vendor>/ccusage/index.js", "daily", "--json", "--since", "2026-04-01"]`
- **AND** no shell subprocess is invoked at any point

#### Scenario: Spawning error produces warning and empty string
- **GIVEN** the `ccusage` child process exits with a non-zero code or cannot be spawned
- **WHEN** `execFile`'s callback fires with an error
- **THEN** a warning MUST be written to stderr in the format `warning: {toolName} fetch failed ({error.message}), showing zero data`
- **AND** the wrapper Promise MUST resolve with `""` (empty string)
- **AND** the calling `fetchHistory`/`fetchTotals` MUST return `EMPTY` totals or `[]` accordingly

#### Scenario: Shell metacharacters in prefixArgs are passed literally
- **GIVEN** a `prefixArgs` entry containing spaces or quote characters (e.g., `"/path with spaces/index.js"`)
- **WHEN** `execFile` is invoked
- **THEN** the argument MUST be passed as a single literal argv entry
- **AND** no shell parsing or quote processing is applied

### Requirement: Output Format Dispatch

The CLI MUST support four output formats: `table` (the existing ANSI-rendered default), `json`, `csv`, and `md`. The format is selected by mutually exclusive global flags: `--json`, `--csv`, `--md`. The default when none is set is `table`.

`parseGlobalFlags` in `src/node/core/cli.ts` MUST return an `outputFormat` field of type `"table" | "json" | "csv" | "md"` (or equivalent). The existing `jsonFlag` boolean MAY remain for internal compatibility during the transition, but all dispatch paths SHALL read from `outputFormat`.

Dispatch functions (`dispatchAllHistory`, `dispatchAllSnapshot`, `dispatchSingleTool`) MUST branch on `outputFormat` and call the appropriate emitter: `emitJson` for `json`, a new `emitCsv` for `csv`, a new `emitMarkdown` for `md`, and the existing `print*` functions for `table`.

#### Scenario: Default invocation uses table format
- **GIVEN** `tu` invoked with no output-format flag
- **WHEN** parsing completes
- **THEN** `outputFormat` MUST be `"table"`
- **AND** dispatch MUST call the existing ANSI-rendered `print*` functions

#### Scenario: --csv selects CSV format
- **GIVEN** `tu cc --csv` invoked
- **WHEN** dispatch runs
- **THEN** `outputFormat` MUST be `"csv"`
- **AND** `emitCsv` MUST be called instead of `printHistory`
- **AND** no ANSI escape codes appear in stdout

#### Scenario: --md selects Markdown format
- **GIVEN** `tu m --md` invoked
- **WHEN** dispatch runs
- **THEN** `outputFormat` MUST be `"md"`
- **AND** `emitMarkdown` MUST be called instead of `printTotal`
- **AND** stdout begins with a `## ` heading line

### Requirement: Output Format Flag Conflicts

The CLI MUST reject invocations that combine incompatible output-format flags. The following combinations MUST produce an error on stderr and exit code 1:

- Any two of `--json`, `--csv`, `--md` together
- Any of `--json`, `--csv`, `--md` combined with `--watch` (or `-w`)

The existing `--watch` + `--json` error SHALL be retained as-is; new errors follow the same pattern (`Error: {flag-a} and {flag-b} are incompatible`).

#### Scenario: --csv + --json is rejected
- **GIVEN** `tu --csv --json` invoked
- **WHEN** `parseGlobalFlags` runs
- **THEN** stderr MUST contain `Error: --json and --csv are incompatible` (or equivalent; exact flag order in the message MAY reflect argv order)
- **AND** the process MUST exit with code 1

#### Scenario: --md + --watch is rejected
- **GIVEN** `tu --md --watch` invoked
- **WHEN** `parseGlobalFlags` runs
- **THEN** stderr MUST contain an error indicating the incompatibility
- **AND** the process MUST exit with code 1

#### Scenario: Existing --json + --watch rejection preserved
- **GIVEN** `tu --json --watch` invoked
- **WHEN** `parseGlobalFlags` runs
- **THEN** stderr MUST contain `Error: --watch and --json are incompatible`
- **AND** the process MUST exit with code 1

### Requirement: `tu completions <shell>` Subcommand

The CLI MUST dispatch a new `completions` subcommand before grammar parsing, at the same dispatch site as existing non-data commands (`init-conf`, `init-metrics`, `sync`, `status`, `update`).

`runCompletions(shell?)` MUST behave as follows:
- `undefined` or no shell argument → print usage (see Install Examples below) to stdout, exit 0
- `"bash"`, `"zsh"`, or `"fish"` → write the corresponding static completion script to stdout, exit 0
- Any other string → print `Unknown shell: {shell}. Supported: bash, zsh, fish` to stderr, exit 1

Completion scripts MUST be statically generated (hardcoded strings in the bundle); they MUST NOT invoke `tu` at tab-press time to enumerate tokens.

#### Scenario: `tu completions bash` emits bash script
- **GIVEN** `tu completions bash` invoked
- **WHEN** dispatch runs
- **THEN** stdout MUST contain a bash completion script that uses the `complete` builtin
- **AND** the script MUST reference the `tu` command
- **AND** the process MUST exit 0

#### Scenario: `tu completions zsh` emits zsh script
- **GIVEN** `tu completions zsh` invoked
- **WHEN** dispatch runs
- **THEN** stdout MUST contain a zsh completion script using `#compdef tu` and `_arguments`/`_values`
- **AND** the process MUST exit 0

#### Scenario: `tu completions fish` emits fish script
- **GIVEN** `tu completions fish` invoked
- **WHEN** dispatch runs
- **THEN** stdout MUST contain a fish completion script using `complete -c tu -n ...` directives
- **AND** the process MUST exit 0

#### Scenario: `tu completions` with no argument prints usage
- **GIVEN** `tu completions` invoked with no further arguments
- **WHEN** dispatch runs
- **THEN** stdout MUST contain `Usage: tu completions <bash|zsh|fish>`
- **AND** stdout MUST contain install examples for all three shells
- **AND** the process MUST exit 0

#### Scenario: Unknown shell returns error
- **GIVEN** `tu completions powershell` invoked
- **WHEN** dispatch runs
- **THEN** stderr MUST contain `Unknown shell: powershell. Supported: bash, zsh, fish`
- **AND** the process MUST exit with code 1

### Requirement: Completion Script Coverage

Each emitted completion script MUST cover the full grammar. Specifically:

- **Non-data subcommands**: `help`, `init-conf`, `init-metrics`, `sync`, `status`, `update`, `completions`
- **Sources**: `cc`, `codex`, `co`, `oc`, `all`
- **Periods**: `d`, `m`, `daily`, `monthly`
- **Display tokens**: `h`, `history`, `dh`, `mh`
- **Global flags (long)**: `--json`, `--csv`, `--md`, `--sync`, `--fresh`, `--watch`, `--interval`, `--user`, `--by-machine`, `--no-color`, `--no-rain`, `--version`, `--help`
- **Global flags (short)**: `-f`, `-w`, `-i`, `-u`, `-v`, `-V`, `-h`
- **`completions` args**: `bash`, `zsh`, `fish`

#### Scenario: Bash script completes `tu c<TAB>` with source + subcommand candidates
- **GIVEN** a shell with the bash completion script sourced
- **WHEN** the user types `tu c` and presses Tab
- **THEN** candidates MUST include at least `cc`, `codex`, `co`, and `completions`

#### Scenario: All three scripts reference the canonical flag list
- **GIVEN** each of the three generated scripts (bash, zsh, fish)
- **WHEN** the script content is inspected
- **THEN** each script MUST contain literal occurrences of every long flag in the taxonomy above

## Display: Formatting

### Requirement: CSV Output Rendering

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

#### Scenario: Snapshot CSV has tool, tokens, input, output, cost columns
- **GIVEN** `tu --csv` invoked with multi-tool snapshot data
- **WHEN** `emitCsv` renders
- **THEN** the header line MUST be `tool,tokens,input,output,cost`
- **AND** each subsequent line MUST contain one tool's values in that column order
- **AND** when more than one tool has visible data, a final `Total,...` row MUST follow

#### Scenario: History CSV has date, token-breakdown, total, cost columns
- **GIVEN** `tu cc h --csv` invoked with single-tool history
- **WHEN** `emitCsv` renders
- **THEN** the header MUST be `date,input,output,cache_write,cache_read,total,cost`
- **AND** each subsequent row MUST have the date in ISO format (`YYYY-MM-DD` for daily, `YYYY-MM` for monthly)

#### Scenario: Total-history pivot CSV has date, per-tool, total columns
- **GIVEN** `tu h --csv` invoked with all-tools history
- **WHEN** `emitCsv` renders
- **THEN** the header MUST be `date,{tool1},{tool2},...,total` where `{toolN}` is the tool's display name
- **AND** rows are sorted by date ascending

#### Scenario: Machine columns append when --by-machine is active
- **GIVEN** `tu --csv --by-machine` invoked
- **WHEN** `emitCsv` renders snapshot data with machine breakdowns
- **THEN** additional columns MUST follow the base columns, named `machine_{name}_cost` for each machine
- **AND** machine columns MUST be sorted alphabetically by machine name

#### Scenario: String with comma is quoted
- **GIVEN** a tool name contains a comma (hypothetical)
- **WHEN** the tool row is rendered in CSV
- **THEN** the tool name field MUST be wrapped in `"` quotes

### Requirement: Markdown Output Rendering

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

#### Scenario: Snapshot Markdown has heading + GFM table
- **GIVEN** `tu m --md` invoked
- **WHEN** `emitMarkdown` renders
- **THEN** the first line MUST be `## Combined Usage (monthly)`
- **AND** the third line MUST be a GFM header row: `| Tool | Tokens | Input | Output | Cost |` (or similar column set)
- **AND** the fourth line MUST be the alignment separator row with left/right alignments

#### Scenario: Total row rendered in bold when multiple tools visible
- **GIVEN** `tu --md` with at least two tools having non-zero totals
- **WHEN** `emitMarkdown` renders
- **THEN** the final table row MUST have `**Total**` as its first cell
- **AND** numeric values in the Total row MUST be bolded (`**value**`) to match the ANSI renderer's `boldWhite` convention

#### Scenario: Total row omitted when only one tool has visible data
- **GIVEN** `tu --md` with exactly one tool having non-zero totals
- **WHEN** `emitMarkdown` renders
- **THEN** no `**Total**` row MUST be rendered
- **AND** this matches the existing `renderTotal` behaviour (total row guarded by `visibleCount > 1`)

#### Scenario: History Markdown date column is left-aligned
- **GIVEN** `tu cc h --md` invoked
- **WHEN** `emitMarkdown` renders
- **THEN** the first column MUST be `Date` with left-alignment (`:---`)
- **AND** all numeric columns (input, output, cache write, cache read, total, cost) MUST be right-aligned (`---:`)

#### Scenario: Machine columns append when --by-machine is active
- **GIVEN** `tu --md --by-machine` invoked
- **WHEN** `emitMarkdown` renders
- **THEN** per-machine cost columns MUST follow the base columns, headed by the machine name (not the A/B/C letter codes used by the ANSI renderer)
- **AND** a `Machines: A = name, B = name` legend line MUST NOT be emitted (machines are named directly in the Markdown header)

### Requirement: Shared Dispatch Layer

The three output formats (`json`, `csv`, `md`) and the default `table` path MUST be selectable via a single decision point per dispatch function. Duplication of fetch logic across format branches is prohibited — fetching happens once, rendering dispatches on `outputFormat`.

#### Scenario: Fetch runs once regardless of format
- **GIVEN** `tu h --csv` invoked
- **WHEN** `dispatchAllHistory` runs
- **THEN** `fetchAllHistory` or `fetchToolMerged` is called exactly once per tool
- **AND** the returned data is passed to `emitCsv` without re-fetching

## Sync: Multi-Machine

### Requirement: Git Invocation via execFile

All git command invocations in `src/node/sync/sync.ts` MUST use `execFile("git", [...argv])` rather than `exec("git -C ... ...")`. The command string and its quoting are replaced by an argv array passed literally.

The `git` helper wrapper in `syncMetrics` MUST accept an argv array (e.g., `(args: string[]) => execFileAsync("git", args)`) rather than a string. Callsites MUST pass arrays:

- `git(["-C", metricsDir, "add", `${user}/`])`
- `git(["-C", metricsDir, "status", "--porcelain", `${user}/`])`
- `git(["-C", metricsDir, "commit", "-m", `# ${user}: update ${date}`])`
- `git(["-C", metricsDir, "pull", "--rebase", "origin", "main"])`
- `git(["-C", metricsDir, "push"])`
- `git(["-C", metricsDir, "rebase", "--abort"])`

The interrupted-rebase recovery path and the single-retry push path MUST preserve their existing semantics (same git commands, same ordering, same error messages on stderr).

#### Scenario: Git commit uses execFile
- **GIVEN** `syncMetrics` called with a dirty working tree
- **WHEN** the commit step runs
- **THEN** `execFile("git", ["-C", metricsDir, "commit", "-m", "# user: update 2026-04-23"])` MUST be invoked
- **AND** no shell is forked

#### Scenario: metricsDir with spaces works correctly
- **GIVEN** `config.metricsDir` is `/home/user/My Data/metrics`
- **WHEN** `syncMetrics` runs git commands
- **THEN** each `execFile` call MUST pass the path as a single literal argv entry
- **AND** all git commands MUST succeed (no path-parsing breakage)

#### Scenario: Interrupted rebase recovery still works
- **GIVEN** `metricsDir/.git/rebase-merge` or `metricsDir/.git/rebase-apply` exists at sync start
- **WHEN** `syncMetrics` detects the marker
- **THEN** `execFile("git", ["-C", metricsDir, "rebase", "--abort"])` MUST be invoked
- **AND** a warning MUST be written to stderr (`Warning: recovering from interrupted rebase`)
- **AND** sync MUST proceed with the normal flow

#### Scenario: Push retry preserved
- **GIVEN** the first `git push` fails
- **WHEN** `syncMetrics` handles the error
- **THEN** a second `execFile("git", ["-C", metricsDir, "push"])` MUST be attempted
- **AND** if the retry also fails, a stderr warning MUST be emitted with the error reason
- **AND** `syncMetrics` MUST return `false`

## Design Decisions

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

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Bundle perf #2 + feature #6 + feature #1 into one change | Confirmed from intake #1; user explicitly requested bundling | S:100 R:70 A:100 D:100 |
| 2 | Certain | `TOOLS` shape: `{ name, binary, prefixArgs, needsFilter }` | Confirmed from intake #2; shape proposed and accepted in discuss | S:95 R:70 A:95 D:95 |
| 3 | Certain | `--csv` and `--md` apply to all three data kinds (snapshot, history, total-history) | Confirmed from intake #3 | S:95 R:75 A:90 D:95 |
| 4 | Certain | `tu completions <shell>` emits to stdout, no auto-install | Confirmed from intake #4 | S:95 R:85 A:90 D:95 |
| 5 | Certain | CSV/MD output strips ANSI, drops inline bars, drops delta arrows | Confirmed from intake #5 | S:100 R:85 A:95 D:100 |
| 6 | Certain | Machine columns preserved under `--by-machine` (CSV: suffixed columns; MD: named columns) | Confirmed from intake #6; rendering detail (letter vs name) settled at spec stage | S:90 R:80 A:90 D:90 |
| 7 | Certain | Supported shells: bash, zsh, fish | Confirmed from intake #7 | S:90 R:85 A:90 D:95 |
| 8 | Certain | `--csv` / `--md` / `--json` / `--watch` mutual-exclusion matrix | Confirmed from intake #8; spec adds scenario coverage | S:90 R:85 A:95 D:95 |
| 9 | Certain | Markdown output includes a leading `## {title}` heading | Confirmed from intake #9 | S:90 R:85 A:85 D:90 |
| 10 | Certain | CSV: RFC 4180, header row, LF, no BOM | Upgraded from intake Confident — Unix convention is the clear default, no realistic alternative for this audience | S:85 R:90 A:95 D:90 |
| 11 | Certain | MD: GFM tables, numeric right-aligned, string left-aligned | Upgraded from intake Confident — GFM is the paste target; alignment mirrors existing ANSI convention | S:85 R:85 A:90 D:90 |
| 12 | Certain | `tu completions` with no arg prints usage (does not auto-detect `$SHELL`) | Upgraded from intake Confident — Design Decision #3 documents rationale; no countervailing signal | S:85 R:90 A:85 D:90 |
| 13 | Certain | Completion scripts are statically generated (hardcoded string constants in the bundle) | Upgraded from intake Confident — Design Decision #4 settles this; dynamic lookup has no realistic advocate | S:85 R:90 A:90 D:90 |
| 14 | Certain | `TOOLS` field naming: `prefixArgs` | Upgraded from intake Confident — Design Decision #1 rationale; no pending dissent | S:80 R:95 A:90 D:90 |
| 15 | Certain | Markdown machine columns use machine names directly (not A/B/C letter codes with legend) | New at spec stage — Design Decision #6. Letter codes exist for ANSI width-conservation; GFM doesn't need them | S:85 R:90 A:90 D:85 |
| 16 | Certain | CSV machine columns are named `machine_{name}_cost` and sorted alphabetically | New at spec stage — machine-readable convention; snake_case, lowercase, deterministic ordering | S:85 R:90 A:85 D:85 |
| 17 | Certain | CSV numeric fields render raw (no thousands separators); MD numeric fields use commas | New at spec stage — CSV targets machines (raw is easier to parse); MD targets humans (readable) | S:85 R:90 A:90 D:85 |
| 18 | Certain | CSV cost fields: no `$`, two decimals; MD cost fields: `$` prefix, two decimals | New at spec stage — same rationale as #17 (machine vs. human readability) | S:85 R:90 A:90 D:85 |
| 19 | Certain | CSV date labels use ISO format (`YYYY-MM-DD` daily, `YYYY-MM` monthly) | Matches existing `normalizeLabel` output; no conversion needed | S:90 R:90 A:95 D:95 |
| 20 | Certain | No backwards-compat shim for old `ToolConfig.command` field | Non-goal per spec; `TOOLS` is internal, no external consumer | S:90 R:80 A:95 D:90 |
| 21 | Certain | `--watch` + any of `--json`/`--csv`/`--md` rejected at parse time | Mirrors existing `--watch` + `--json` rejection; single consistent pattern | S:90 R:85 A:95 D:95 |
| 22 | Certain | Error message format for flag conflicts: `Error: {flag-a} and {flag-b} are incompatible` | Matches existing `Error: --watch and --json are incompatible` message | S:90 R:90 A:95 D:95 |
| 23 | Confident | Heading titles in MD exactly match ANSI renderer titles (`Combined Usage (daily)`, `Claude Code (monthly)`, `Combined Cost History (daily)`) | Consistency with existing formatter output; readers familiar with one format recognise titles in the other | S:75 R:85 A:85 D:80 |
| 24 | Confident | `maxBuffer` stays at 10MB (unchanged) | Streaming via `spawn` is explicitly a non-goal; bumping or removing the cap is out of scope for this change | S:80 R:85 A:90 D:85 |
| 25 | Confident | New renderers (`emitCsv`, `emitMarkdown`) live in `src/node/tui/formatter.ts` alongside existing renderers | Co-located with existing display logic; formatter.ts is already the rendering module. If file size becomes unwieldy, splitting is a trivial follow-up | S:65 R:95 A:80 D:75 |
| 26 | Confident | Completion scripts live in a new `src/node/core/completions.ts` module | Isolated string constants; keeps `cli.ts` (already 1105 lines) from growing further. Pure data module | S:65 R:95 A:80 D:75 |

26 assumptions (22 certain, 4 confident, 0 tentative, 0 unresolved).
