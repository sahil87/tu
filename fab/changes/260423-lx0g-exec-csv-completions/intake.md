# Intake: Safer process spawning, CSV/Markdown export, and shell completions

**Change**: 260423-lx0g-exec-csv-completions
**Created**: 2026-04-23
**Status**: Draft

## Origin

> Three bundled improvements: (1) Switch exec() to execFile() with argv arrays in fetcher.ts and sync.ts — change TOOLS registry to carry { binary, args[] }. Removes shell fork per call and shell-injection surface. (2) Add --csv and --md output flags parallel to --json, applying to every data command (snapshot, history, total-history) via a single dispatch layer above existing renderers. Strip ANSI, drop bars, keep machine columns. (3) Add `tu completions <shell>` command emitting bash/zsh/fish completion scripts to stdout for user eval/sourcing.

This change emerged from a `/fab-discuss` session on 2026-04-23 that surveyed the codebase for performance improvements and new features. The agent produced a categorised list (8 performance items, 17 feature suggestions); the user selected a subset across both categories, then narrowed further to the three items bundled here because they touch adjacent code paths:

- **Perf item #2** — switch `exec()` to `execFile()` in `fetcher.ts` and `sync.ts`
- **Feature item #6** — `--csv` and `--md` output flags parallel to the existing `--json`
- **Feature item #1** — `tu completions <shell>` emitting bash/zsh/fish scripts to stdout

Other items selected during discussion are deferred to subsequent changes: perf #7 (streaming `spawn` to replace `maxBuffer`), feature #4 (`tu diff`), feature #10 (`tu sessions`), feature #14 (team/org view in multi mode).

Bundling rationale (from user): "All three touch adjacent code… bundle because the blast radius is similar and they'd otherwise be three trivially-small PRs." Specifically, all three modify `TOOLS` registry, `cli.ts` dispatch, or the output-format switch — splitting would re-touch the same files three times.

## Why

### `exec()` → `execFile()`

Three motivations in parallel:

1. **Security**. `exec()` invokes `/bin/sh` to parse the command string, so any interpolation is a latent shell-injection vector. Today we interpolate `config.metricsDir`, `config.user`, `toolKey`, and passthrough flags. None are hostile-user-controlled today, but the pattern is wrong and future data sources or filter flags may not share that guarantee.
2. **Performance**. Every default `tu` invocation spawns three `ccusage*` children in parallel. `exec()` forks a shell that then forks the real binary — a wasted fork per tool per call. Three avoided forks per snapshot invocation. Small but real on cold-start.
3. **Correctness**. `sync.ts` git calls build command strings like `git -C "${metricsDir}" add "${user}/"`. If `metricsDir` contains spaces or quote characters (non-default setups on NFS mounts, WSL paths, `~/"My Data"/…`), the current quoting is fragile. `execFile` drops the shell-parsing step entirely so arguments pass through as literal argv entries.

If we leave this as-is: no imminent breakage, but we accumulate a latent injection surface and pay a small startup tax on the most common invocation forever.

### `--csv` and `--md` output flags

The existing `--json` serves programmatic consumers. CSV and Markdown cover two distinct use cases JSON serves poorly:

- **CSV** — spreadsheets, BI tools, `awk`/`cut` pipelines. `tu` users (devs tracking AI spend) commonly need to drop numbers into finance-facing reports or billing tools. JSON requires a transform step; CSV is the lingua franca of tabular export.
- **Markdown** — copy-paste into PRs, standups, Slack threads, GitHub issues. The existing ANSI tables render as escape-code soup when pasted. `--md` yields a paste-ready artifact in one step.

Without this: users pipe `--json | jq | paste` or screenshot their terminal. Both work poorly; both defeat automation.

### `tu completions <shell>`

Standard modern-CLI UX. The `tu` grammar is small and regular (sources × periods × display × flags), so completion scripts are cheap to generate and high-value. Without completions, users type `tu --help` and guess flag names — degrading the fast-startup experience the constitution prioritises (Principle IV).

## What Changes

### 1. `TOOLS` registry shape + `execFile` migration

**Files**: `src/node/core/fetcher.ts`, `src/node/core/types.ts`, `src/node/sync/sync.ts`

`ToolConfig` in `types.ts` becomes:

```typescript
export interface ToolConfig {
  name: string;
  binary: string;        // path to executable
  prefixArgs: string[];  // args prepended before period/flags (e.g., ["/path/to/index.js"] for vendor path)
  needsFilter: boolean;
}
```

`TOOLS` registry in `fetcher.ts`:

```typescript
export const TOOLS: Record<string, ToolConfig> = {
  cc: {
    name: "Claude Code",
    binary: useVendor ? "node" : `${BIN}/ccusage`,
    prefixArgs: useVendor ? [`${BIN}/ccusage/index.js`] : [],
    needsFilter: false,
  },
  codex: {
    name: "Codex",
    binary: useVendor ? "node" : `${BIN}/ccusage-codex`,
    prefixArgs: useVendor ? [`${BIN}/ccusage-codex/index.js`] : [],
    needsFilter: true,
  },
  oc: {
    name: "OpenCode",
    binary: useVendor ? "node" : `${BIN}/ccusage-opencode`,
    prefixArgs: useVendor ? [`${BIN}/ccusage-opencode/index.js`] : [],
    needsFilter: true,
  },
};
```

`execAsync` switches from `exec(cmd, opts, cb)` to `execFile(file, args, opts, cb)` in both `fetcher.ts` and `sync.ts`. `runTool` builds argv: `[...tool.prefixArgs, period, "--json", ...extraArgs]`. The Promise wrapper signature changes from `(cmd: string) => Promise<string>` to `(file: string, args: string[]) => Promise<string>`.

`sync.ts` git helper changes similarly:

```typescript
// Before
const git = (args: string) => execAsync(`git -C "${metricsDir}" ${args}`);
await git(`add "${user}/"`);

// After
const git = (args: string[]) => execAsyncFile("git", ["-C", metricsDir, ...args]);
await git(["add", `${user}/`]);
```

### 2. `--csv` and `--md` output flags

**Files**: `src/node/core/cli.ts` (parsing + dispatch), `src/node/tui/formatter.ts` or new renderer module for CSV/MD

**Parsing** in `parseGlobalFlags`: recognise `--csv` and `--md`. Mutually exclusive with each other, with `--json`, and with `--watch` (matching the existing `--json`/`--watch` exclusion). Violations print an error and exit 1.

**Dispatch**: introduce an `OutputFormat` type (`"table" | "json" | "csv" | "md"`) plumbed through the dispatch functions. The current `jsonFlag` branch becomes a switch:

```typescript
switch (outputFormat) {
  case "json": emitJson(data); break;
  case "csv":  emitCsv(data, kind); break;
  case "md":   emitMarkdown(data, kind); break;
  default:     printTotalHistory(period, data, undefined, fmtOpts);
}
```

`kind` is one of `"snapshot" | "history" | "total-history"` so the renderer knows which row shape to produce.

**CSV shape** — RFC 4180 quoting, header row, LF line endings, no BOM.

Snapshot (`tu`, `tu cc`, `tu m`):
```
tool,tokens,input,output,cost
Claude Code,1234567,800000,400000,12.34
Codex,234567,150000,80000,2.45
Total,1469134,950000,480000,14.79
```

Single-tool history (`tu cc h`, `tu oc mh`):
```
date,input,output,cache_write,cache_read,total,cost
2026-04-21,80000,40000,20000,5000,145000,2.34
2026-04-22,90000,50000,25000,6000,171000,2.89
```

Total-history pivot (`tu h`, `tu mh`):
```
date,Claude Code,Codex,OpenCode,total
2026-04-21,2.34,0.50,0.10,2.94
2026-04-22,2.89,0.60,0.15,3.64
```

When `--by-machine` is active, additional columns follow (named `machine_{name}_cost`).

**Markdown shape** — GitHub-flavoured tables. Right-aligned numeric columns via `---:`, left-aligned string columns via `:---`. Leading `## {title}` heading so output pastes directly into PR/issue bodies. No ANSI, no bars, no delta arrows.

Example (snapshot):

```markdown
## Combined Usage (daily)

| Tool        |    Tokens |   Input |  Output |   Cost |
|:------------|----------:|--------:|--------:|-------:|
| Claude Code | 1,234,567 | 800,000 | 400,000 | $12.34 |
| Codex       |   234,567 | 150,000 |  80,000 |  $2.45 |
| **Total**   | 1,469,134 | 950,000 | 480,000 | $14.79 |
```

**Stripping rules** for both formats:
- No ANSI escape codes
- No inline bar chart characters
- No delta indicators (`↑`/`↓`) — they have no meaning outside watch context
- Commas in numeric columns are preserved in Markdown (human-readable) and dropped in CSV (machine-readable)

### 3. `tu completions <shell>`

**Files**: `src/node/core/cli.ts` (dispatch), new string constants for shell scripts (inline or in a new `src/node/core/completions.ts`)

New subcommand dispatch branch in `main()`:

```typescript
if (cmd === "completions") {
  runCompletions(filteredArgs[1]);  // may be undefined
  return;
}
```

`runCompletions(shell?)` behaviour:
- `undefined` or no arg → print usage (`Usage: tu completions <bash|zsh|fish>` + install examples) and exit 0
- `bash` / `zsh` / `fish` → write the corresponding script to stdout, exit 0
- Any other value → stderr `Unknown shell: {shell}. Supported: bash, zsh, fish` and exit 1

**Completable tokens** the script should offer:

- **Non-data subcommands**: `help`, `init-conf`, `init-metrics`, `sync`, `status`, `update`, `completions`
- **Sources**: `cc`, `codex`, `co`, `oc`, `all`
- **Periods**: `d`, `m`, `daily`, `monthly`
- **Display**: `h`, `history`, `dh`, `mh`
- **Global flags**: `--json`, `--csv`, `--md`, `--sync`, `--fresh`, `-f`, `--watch`, `-w`, `--interval`, `-i`, `--user`, `-u`, `--by-machine`, `--no-color`, `--no-rain`, `--version`, `-v`, `-V`, `--help`, `-h`
- **`completions` args**: `bash`, `zsh`, `fish`

**Install examples** shown in usage:

```
bash: echo 'source <(tu completions bash)' >> ~/.bashrc
zsh:  tu completions zsh > "${fpath[1]}/_tu"
fish: tu completions fish > ~/.config/fish/completions/tu.fish
```

## Affected Memory

- `cli/data-pipeline`: (modify) — `TOOLS` shape change (`command: string` → `binary: string` + `prefixArgs: string[]`); `exec()` → `execFile()` migration; add `--csv`/`--md` to global flags taxonomy; add `completions` to non-data subcommand catalogue; record the output-format dispatch layer
- `display/formatting`: (modify) — three new output formats (CSV, Markdown) alongside the existing ANSI table and JSON paths; dispatch-layer selection; strip rules (no ANSI, no bars, no deltas); column shapes per `kind` (snapshot/history/total-history)
- `sync/multi-machine`: (modify) — `execFile()` replacing `exec()` for all git commands; argv construction in `syncMetrics` and the fetch portion of `fullSync`

## Impact

- **Source files changed**: `src/node/core/fetcher.ts`, `src/node/core/types.ts`, `src/node/core/cli.ts`, `src/node/sync/sync.ts`, `src/node/tui/formatter.ts` (or new module for CSV/MD renderers); new file for completion scripts (recommended: `src/node/core/completions.ts`)
- **Tests**: argv construction for `execFile` in fetcher and sync; CSV output for all three kinds (snapshot, history, total-history) with and without `--by-machine`; Markdown output for the same; completions output for each shell (snapshot compare); flag-conflict errors (`--json` + `--csv`, `--md` + `--watch`, etc.)
- **Bundle size**: three completion scripts as string constants. Estimated ~60 lines (bash), ~80 (zsh), ~40 (fish), ~5KB raw / ~3KB after esbuild minification. Acceptable against Principle III (single-bundle distribution).
- **No new dependencies**. No `package.json` change.
- **User-facing compatibility**: purely additive. Existing `--json`, `--watch`, subcommands, and output formats unchanged. The `TOOLS` shape change is private (internal to the bundled binary; no external consumer).
- **New flag conflicts**: `--csv` / `--md` / `--json` mutually exclusive with each other; `--csv` / `--md` / `--json` all incompatible with `--watch`. Error messages modeled on the existing `--watch + --json` conflict.

## Open Questions

- Should the markdown heading (`## Combined Usage (daily)`) always render, or be suppressed by a `--no-heading` flag for users who want pure tables? Current lean: always render. Adding a suppression flag is easy to follow up on if requested.
- For `tu completions` without a shell arg, print usage vs. auto-detect via `$SHELL`? Leaning toward usage — completion installation is a one-off operation and explicit is safer than magic (silent mismatch if `$SHELL` doesn't match the shell the user is actually running).
- Does the completion script need to dynamically call `tu` to enumerate valid sources/tools, or is static generation sufficient? Lean toward static — the taxonomy is stable, dynamic is costly (spawns `tu` on every tab-press), and a stale script only matters if the taxonomy grows (which is a known-at-release-time event).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Bundle perf #2 + feature #6 + feature #1 into a single change | User explicitly requested bundling: "kick off a fab-new for perf2+feature6+feature1" | S:100 R:70 A:100 D:100 |
| 2 | Certain | `TOOLS` shape becomes `{ name, binary, prefixArgs, needsFilter }` | User accepted the exact shape proposed during `/fab-discuss` ("Needs `TOOLS` to carry `{binary, args[]}` instead of `command: string`") | S:90 R:65 A:90 D:90 |
| 3 | Certain | `--csv` and `--md` apply to every data command (snapshot, history, total-history), not only history | User accepted agent's recommendation "I'd vote all, with a single dispatch layer" in `/fab-discuss` | S:95 R:70 A:90 D:95 |
| 4 | Certain | `tu completions <shell>` emits script to stdout; no auto-install, no write-to-disk | User accepted the stdout-emit pattern over handwritten static scripts in `/fab-discuss` | S:95 R:80 A:90 D:95 |
| 5 | Certain | CSV/MD output strips ANSI, drops inline bars, drops delta arrows | Explicit wording of the user-approved proposal ("Strip ANSI, drop bars, keep machine columns if present") | S:100 R:80 A:95 D:100 |
| 6 | Certain | Machine columns preserved when `--by-machine` is active (CSV and MD) | Explicit wording of the user-approved proposal | S:100 R:80 A:95 D:100 |
| 7 | Certain | Supported shells: bash, zsh, fish (no PowerShell, nushell, etc.) | Agent proposed exact list "(bash/zsh/fish)" in `/fab-discuss`; user accepted without objection or addition | S:90 R:80 A:90 D:90 |
| 8 | Certain | `--csv` / `--md` / `--json` mutually exclusive with each other and with `--watch` | Mirrors existing `--json`/`--watch` exclusion documented in `cli/data-pipeline.md`; only one output format makes sense per invocation | S:90 R:80 A:95 D:95 |
| 9 | Certain | Markdown output includes a leading `## {title}` heading (e.g., `## Combined Usage (daily)`) | User-approved proposal showed this exact heading format in the example; no dissent raised | S:90 R:85 A:85 D:90 |
| 10 | Confident | CSV: RFC 4180 quoting, header row, LF line endings, no BOM | Unix-tool convention; most pipeline-friendly shape. Alternatives (CRLF, BOM) would be surprising for the target audience | S:80 R:85 A:90 D:85 |
| 11 | Confident | Markdown: GitHub-flavoured tables; right-aligned numeric columns, left-aligned string columns | Target paste contexts (GH PRs, Slack, internal docs) all render GFM. Alignment mirrors existing ANSI-table convention | S:80 R:80 A:85 D:85 |
| 12 | Confident | `tu completions` without a shell arg prints usage (does not auto-detect `$SHELL`) | One-off operation, explicit is safer than magic; `$SHELL` may not match the shell the user is actually running | S:65 R:85 A:80 D:80 |
| 13 | Confident | Completion script is statically generated (not dynamically invoking `tu` per tab-press) | Taxonomy is stable; dynamic lookup is costly on every completion; re-release updates the script when the taxonomy changes | S:70 R:85 A:85 D:85 |
| 14 | Confident | `TOOLS` field naming: `prefixArgs` (not `args` or `preArgs`) | Needs to be distinct from per-call `extraArgs`; "prefix" semantics match the intent (prepended before period/flags) | S:55 R:95 A:85 D:70 |

14 assumptions (9 certain, 5 confident, 0 tentative, 0 unresolved).
