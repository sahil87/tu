# Intake: Usage-Error Exit Codes

**Change**: 260717-8h6g-usage-error-exit-codes
**Created**: 2026-07-18

## Origin

One-shot invocation: `/fab-new 8h6g` (backlog ID, no prior conversation context). Backlog item from `fab/backlog.md`:

> [8h6g] 2026-07-18: Align tu's usage-error exit codes with the toolkit convention `0` success / `1` operational failure / `2` usage error (principle №4, `shll standards principles`). Today tu exits `1` for BOTH operational failures AND usage errors. USAGE-ERROR sites that should move to exit `2` (all in src/node/core/cli.ts unless noted): `Unknown argument` (parseDataArgs catch in main), `Unknown tool` (dispatchSingleTool), `Unknown shell` (runShellInit), the format-incompatibility errors (`--json`/`--csv`/`--md`/`--watch` matrix in parseGlobalFlags), `-u requires a username`, `--interval requires a numeric value` / min / max, `--since`/`--until requires a date` and inverted-window. Leave OPERATIONAL failures at `1`: sync failures, metrics_repo not set, clone failures, brew update/upgrade failures (runSync, runInitMetrics, runUpdate). WHY DEFERRED (not a small additive fix): it is a systematic change across ~14 call sites, changes a MACHINE-OBSERVABLE contract that scripts branch on, and requires updating ~20+ existing test assertions that pin `exitCode === 1` / `code === 1` for usage-error paths (cli-parser.test.ts lines ~271-437, cli-watch-flag.test.ts, completions.test.ts unknown-shell). Best done as its own reviewable unit with the exit-code table documented per-subcommand. Constitution Output Stability: exit-code changes are a machine-format change — ride a minor version bump. Deferred from change 260717-rdo3 (toolkit standards conformance audit).

All enumerated sites were verified against the current code at intake time; the actual count is **16 usage-error sites** (the backlog's "~14" undercounted the 6-row format matrix and the 3 interval exits).

## Why

1. **The pain point**: tu exits `1` for every failure, so a caller cannot distinguish "I invoked the tool wrong" (fix the arguments) from "the operation failed" (retry, check network, check config). Toolkit principle №4 (*Fail fast with actionable errors*, `shll standards principles`) mandates the convention `0` success / `1` operational failure / `2` usage error, with exit codes documented per subcommand. The tu constitution's **Toolkit Standards** section binds this repo to that standard.
2. **Consequence of not fixing**: these tools are operated by AI agents at least as often as humans; an agent branching on exit code takes the wrong recovery path (retrying a malformed invocation, or "fixing" arguments that were fine). Every other toolkit CLI converging on the convention makes tu the odd one out.
3. **Why this approach**: flip only the exit code at the enumerated usage-error sites, leave message text and stderr routing byte-identical, update the pinned test assertions, and document the per-subcommand exit-code table in the CLI-surface spec. This is deliberately a standalone reviewable unit (deferred out of 260717-rdo3 for exactly that reason) because it changes a machine-observable contract.

## What Changes

### 1. Usage-error sites move from exit 1 to exit 2 — `src/node/core/cli.ts`

All 16 sites, verified against current code (line numbers indicative):

| # | Function | Error message | Line |
|---|----------|--------------|------|
| 1 | `runShellInit` | `Unknown shell: {shell}. Supported: bash, zsh, fish` | ~315 |
| 2 | `parseGlobalFlags` | `Error: --interval requires a numeric value` | ~708 |
| 3 | `parseGlobalFlags` | `Error: --interval minimum is 5 seconds` | ~713 |
| 4 | `parseGlobalFlags` | `Error: --interval maximum is 3600 seconds` | ~717 |
| 5 | `parseGlobalFlags` | `Error: --watch and --json are incompatible` | ~726 |
| 6 | `parseGlobalFlags` | `Error: --json and --csv are incompatible` | ~730 |
| 7 | `parseGlobalFlags` | `Error: --json and --md are incompatible` | ~734 |
| 8 | `parseGlobalFlags` | `Error: --csv and --md are incompatible` | ~738 |
| 9 | `parseGlobalFlags` | `Error: --watch and --csv are incompatible` | ~742 |
| 10 | `parseGlobalFlags` | `Error: --watch and --md are incompatible` | ~746 |
| 11 | `parseGlobalFlags` | `Error: -u requires a username` | ~751 |
| 12 | `parseGlobalFlags` | `Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)` | ~758 |
| 13 | `parseGlobalFlags` | `Error: --until requires a date (YYYY-MM-DD or YYYYMMDD)` | ~765 |
| 14 | `parseGlobalFlags` | `Error: --since must be on or before --until` | ~770 |
| 15 | `dispatchSingleTool` | `Unknown tool: {toolKey}` (+ `SHORT_USAGE` to stderr) | ~966 |
| 16 | `main` (parseDataArgs catch) | `Unknown argument: ...` (thrown message + `SHORT_USAGE` to stderr) | ~1336 |

**Error message text and stderr routing are byte-identical — only the exit code changes.**

Introduce a named mechanism instead of scattering literal `2`s (code-quality.md: no magic numbers without named constants) — either a `const EXIT_USAGE = 2` or a small `usageError(msg): never` helper that prints to stderr and exits 2. Exact shape is decided at apply; note sites 15/16 additionally print `SHORT_USAGE`, so a plain constant may compose more simply than a helper.

### 2. Operational failures stay at exit 1 (verified, unchanged)

| Function | Failure | Line |
|----------|---------|------|
| `runInitMetrics` | `metrics_repo` not set | ~189 |
| `runInitMetrics` | metrics dir exists but is not a git repo | ~204 |
| `runUpdate` | `brew update` failed | ~333 |
| `runUpdate` | could not determine latest version (`brew info`) | ~348 |
| `runUpdate` | `brew upgrade` failed | ~362 |
| `runSync` | `metrics_repo` not set | ~437 |
| `runSync` | auto-clone failed / metricsDir missing | ~442 |
| `runSync` | sync failed | ~448 |
| `main().catch` | catch-all for unexpected thrown errors | ~1433 |

`src/node/tui/watch.ts` `process.exit(0)` (quit key) is untouched.

### 3. Test updates — ~21 pinned assertions across 3 files

All existing usage-error tests pin exit `1` and must move to `2` (Test Integrity: spec is source of truth; tests follow):

- `src/node/core/__tests__/cli-parser.test.ts` — **14 ×** `assert.equal(s.code, 1)` → `2`: the format-conflict matrix (including short-form `-w` variants and `-j` alias variants) and the `--since`/`--until` validation-error block. Four `it()` titles literally say "exits 1" — rename to "exits 2".
- `src/node/core/__tests__/cli-watch-flag.test.ts` — **6 ×** `assert.equal(s.code, 1)` → `2`: interval missing/non-numeric/fractional/min/max, `--watch`+`--json`.
- `src/node/core/__tests__/completions.test.ts` — **1 ×** `assert.equal(cap.exitCode, 1)` → `2` (unknown shell). The three `assert.notEqual(cap.exitCode, 1)` success-path assertions remain valid as-is.
- **Untouched**: `cli-sync.test.ts`, `cli-init-metrics.test.ts` — their exit-`1` assertions cover operational paths and now pin the 1-vs-2 split from the other side.
- **Coverage gaps to close while here** (SHOULD per code-review.md — output changes carry test coverage): no existing exit-code test covers `Unknown tool`, `Unknown argument`, or `-u requires a username`. Add exit-2 assertions for at least unknown tool and unknown argument.

### 4. Per-subcommand exit-code table — `docs/specs/usage.md`

Principle №4 requires exit codes documented per subcommand in the tool's CLI-surface spec; for tu that is `docs/specs/usage.md`. Add an `## Exit Codes` section stating the convention (`0` success / `1` operational failure / `2` usage error) and a per-subcommand table, approximately:

| Command | 0 | 1 | 2 |
|---------|---|---|---|
| `tu [source] [period] [display]` (data commands, incl. `--watch`) | success | unexpected runtime error | unknown argument/tool, bad flag value, incompatible format flags |
| `tu sync` | success | metrics_repo unset, clone/sync failure | — |
| `tu init-metrics` | success | metrics_repo unset, dir not a git repo | — |
| `tu update` | success (incl. non-brew install message, already up to date) | brew update/info/upgrade failure | — |
| `tu shell-init [shell]` | success (script emitted; no-arg usage listing) | — | unknown shell |
| `tu init-conf`, `tu status`, `tu help` | success | unexpected runtime error | — |

(Exact wording/rows finalized at apply against the real dispatch list.)

### 5. Version / release

Exit codes are a machine-observable contract — per the constitution's Output Stability rule this rides a **minor version bump** (0.9.x → 0.10.0). The release flow owns the bump (`just release minor` → `scripts/release.sh`); this change does not edit `package.json`. The PR description must flag: *next release must be `just release minor`, not patch*.

## Affected Memory

- `cli/data-pipeline`: (modify) — several requirement bullets currently pin "exits 1" for usage-error paths (the format-incompatibility bullet, the `--since`/`--until` validation bullet, the `shell-init` unknown-shell bullet). Update those to exit 2 and record the 0/1/2 convention with the usage-vs-operational site classification.

## Impact

- `src/node/core/cli.ts` — 16 exit-code flips + named constant/helper.
- `src/node/core/__tests__/cli-parser.test.ts`, `cli-watch-flag.test.ts`, `completions.test.ts` — ~21 assertion updates, ~4 test-title renames, a few new coverage tests.
- `docs/specs/usage.md` — new `## Exit Codes` section.
- `docs/memory/cli/data-pipeline.md` — updated at hydrate.
- **Machine contract**: downstream scripts branching on exit codes see usage errors move 1 → 2 (the entire point). Breaking for any consumer pinning `=== 1` on usage-error paths — hence the minor bump.
- `fab/backlog.md` — item [8h6g] marked done at archive time (handled by `/fab-archive`).

## Open Questions

(none — the backlog item fully enumerates scope; all sites and test assertions were verified against the current code at intake time)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Site classification: exactly the 16 usage-error sites move to exit 2; the 9 operational sites and `watch.ts` exit(0) stay unchanged | Backlog enumerates the split explicitly; every site verified against current cli.ts at intake | S:95 R:85 A:95 D:90 |
| 2 | Certain | Error message text and stderr routing stay byte-identical; only exit codes change | Backlog scopes the change to exit codes; principle №2 already satisfied by current messages | S:90 R:90 A:90 D:90 |
| 3 | Certain | The ~21 exit-1 test assertions move to exit 2; operational-path tests untouched | Constitution Test Integrity: spec is source of truth, tests verify conformance | S:85 R:90 A:95 D:90 |
| 4 | Confident | Use a named constant/helper (`EXIT_USAGE = 2` or `usageError()`) instead of literal `2`s; exact shape decided at apply | code-quality.md forbids magic numbers; choice between const and helper is low-stakes and reversible | S:40 R:90 A:75 D:60 |
| 5 | Confident | The per-subcommand exit-code table lands in `docs/specs/usage.md` (new `## Exit Codes` section); README and docs/site untouched | Principle №4 names "the tool's CLI-surface spec" — usage.md is tu's; backlog asks for the table without naming a file | S:60 R:85 A:75 D:70 |
| 6 | Confident | No `package.json` version edit in this change; the next release rides `just release minor` and the PR flags it | Release flow (`scripts/release.sh`) owns version bumps; constitution requires minor for machine-format changes | S:70 R:80 A:70 D:65 |
| 7 | Confident | Add missing exit-code coverage for unknown tool / unknown argument while updating tests | code-review.md: output changes SHOULD carry test coverage; small additive tests, easily dropped if review disagrees | S:50 R:90 A:80 D:70 |

7 assumptions (3 certain, 4 confident, 0 tentative, 0 unresolved).
