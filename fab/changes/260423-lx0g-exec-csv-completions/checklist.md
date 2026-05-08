# Quality Checklist: Safer process spawning, CSV/Markdown export, and shell completions

**Change**: 260423-lx0g-exec-csv-completions
**Generated**: 2026-04-23
**Spec**: `spec.md`

## Functional Completeness

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

## Behavioral Correctness

- [ ] CHK-011 Existing `--watch + --json` rejection preserved: Error message still reads `Error: --watch and --json are incompatible` with its original wording
- [ ] CHK-012 Existing ccusage spawn-error semantics preserved: On process error, stderr warning in format `warning: {toolName} fetch failed ({error.message}), showing zero data`; wrapper resolves `""`; `fetchHistory`/`fetchTotals` return `EMPTY` / `[]`
- [ ] CHK-013 Existing interrupted-rebase recovery preserved: `.git/rebase-merge` or `.git/rebase-apply` presence triggers `git rebase --abort` with stderr warning `Warning: recovering from interrupted rebase`
- [ ] CHK-014 Existing push retry preserved: First push failure triggers a second attempt; second failure emits stderr warning with error reason and returns `false`
- [ ] CHK-015 Existing `maxBuffer: 10 * 1024 * 1024` preserved on the fetcher spawn wrapper
- [ ] CHK-016 Existing total-row visibility rule preserved: Total row rendered only when `visibleCount > 1` (applies to CSV and Markdown alike)
- [ ] CHK-017 Existing ANSI `print*` path unchanged: `tu` with no format flag produces byte-identical output to pre-change on the same input (aside from unrelated runtime nondeterminism)

## Removal Verification

- [ ] CHK-018 `ToolConfig.command` field removed: No reference to `tool.command` or `.command:` property access on `ToolConfig` remains anywhere in `src/node/**`
- [ ] CHK-019 String-based `exec(cmd)` call sites removed: grep for `child_process.exec\b` (not `execFile`) in `src/node/core/fetcher.ts` and `src/node/sync/sync.ts` returns zero matches

## Scenario Coverage

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

## Edge Cases & Error Handling

- [ ] CHK-039 CSV string field with embedded comma is wrapped in `"` quotes per RFC 4180
- [ ] CHK-040 CSV string field with embedded `"` is quoted and the inner `"` is doubled (`""`)
- [ ] CHK-041 Empty data (no usage, `allZero`) in CSV and MD emits an empty-data indicator or a header-only table consistent with the existing `No usage` / `No data` ANSI fallback (document the chosen form in tests)
- [ ] CHK-042 Single-tool Markdown output with only one tool visible omits the Total row (mirrors `visibleCount > 1` guard)

## Code Quality

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

## Security

- [ ] CHK-054 No shell subprocess is forked by any new or modified code path in `fetcher.ts` or `sync.ts` (verified by grep for `child_process.exec\b` and by test assertion on the mock's invocation)
- [ ] CHK-055 Argv arrays pass user-controlled strings (config.metricsDir, config.user, toolKey, extraArgs) as literal entries — no string interpolation into a command that a shell would re-parse

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
