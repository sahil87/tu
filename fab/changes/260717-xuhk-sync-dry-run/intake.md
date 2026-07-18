# Intake: Add `--dry-run` to sync

**Change**: 260717-xuhk-sync-dry-run
**Created**: 2026-07-18

## Origin

One-shot `/fab-new xuhk` invocation resolving backlog item `[xuhk]` (fab/backlog.md), raw text:

> Add `--dry-run` to tu's sync mutation (principle №5, visible mutation boundaries — `shll standards principles`). `tu sync` and `tu <cmd> --sync` mutate a shared git metrics repo (writeMetrics + git add/commit/pull --rebase/push in src/node/sync/sync.ts, driven from runSync/fetchToolMerged in cli.ts). The standard requires destructive writes to support an ACCURATE-preview `--dry-run` that shares the real code path (a dry-run that drifts from the live path is worse than none). SCOPE: thread a dryRun flag through fullSync/writeMetrics so it computes what WOULD be written/committed/pushed and prints it (to stdout as the requested preview, or stderr per the stream split — decide at intake) without touching the working tree, the metrics repo, or the network; add `--dry-run` to parseGlobalFlags/GlobalFlags (or as a sync-scoped flag) and to completions.ts (all three shells) + FULL_HELP. Note: tu's sync is ADDITIVE with a never-shrink write guard (see docs/memory/sync/multi-machine.md), so this is preview-for-safety, not destructive-consent — no `--yes` needed. WHY DEFERRED: adding an accurate dry-run means restructuring the sync flow to be preview-capable end-to-end (a feature addition), beyond the audit's small-additive boundary. The mutation is already visibly NAMED (`sync`), so the naming half of №5 already passes. Deferred from change 260717-rdo3 (toolkit standards conformance audit).

Key intake-time findings that resolved the backlog's open points:

- **Stream split resolved by observation**: `shll uninstall --dry-run` — the principles standard's named reference implementation for №5 — prints its preview to **stdout** with an empty stderr and exit 0. Combined with principle №2 ("data the caller asked for goes to stdout"; with `--dry-run` the preview IS the requested data), the preview goes to **stdout**.
- **Combined-invocation trap discovered**: in multi mode, every data fetch (`fetchToolMerged` / `fetchToolMergedWithMachines`, src/node/core/cli.ts) already calls `writeMetrics` *outside* the sync boundary — on every `tu cc`, not just under `--sync`. A `tu cc --sync --dry-run` that "previews then proceeds" would print "would write X" and then write those same day-files for real via the fetch path. Therefore `--dry-run` is honored **only by `tu sync`**; any other invocation carrying it fails fast (see What Changes).

## Why

1. **Problem**: `tu sync` and `tu <cmd> --sync` mutate a *shared* git metrics repo (day-file writes, then `git add`/`commit`/`pull --rebase`/`push`) with no way to see what a sync would do before doing it. Toolkit principle №5 (visible mutation boundaries) makes an accurate-preview `--dry-run` a MUST for destructive writes, and the constitution's Toolkit Standards section binds tu to that standard.
2. **Consequence of not fixing**: tu remains nonconformant with a MUST-level toolkit standard (flagged and deferred by the 260717-rdo3 conformance audit), and agents/users operating a multi-machine setup cannot inspect a pending sync — notably which day-files the never-shrink guard would skip vs. overwrite — without mutating the shared repo.
3. **Approach**: share the real code path (per the standard: "a dry-run that drifts from the live path is worse than none"). The write-decision logic (`isShrinkingWrite` + path construction in `writeMetrics`) runs identically in both modes; only the filesystem/git/network effects are gated. This mirrors the reference implementation (`shll uninstall --dry-run` threads the same single-sourced command builder into preview and live run). Safety framing is preview-for-safety, not destructive-consent: sync is additive with a never-shrink guard, so no `--yes` gate is added (backlog is explicit on this).

## What Changes

### 1. Flag surface: `--dry-run`, parsed globally, honored only by `tu sync`

- Add `dryRunFlag: boolean` to `GlobalFlags` and parse `--dry-run` in `parseGlobalFlags` (src/node/core/cli.ts) — same `rawArgs.includes(...)` + filter-list pattern as `--sync`/`--fresh`. Precedent: `--skip-brew-update` is parsed globally but honored by one command.
- In `main()`: `tu sync --dry-run` routes the flag into `runSync`. **Any other invocation** carrying `--dry-run` (e.g. `tu cc --dry-run`, `tu cc --sync --dry-run`) fails fast on stderr with an actionable message naming the supported form, exit 1 (tu's uniform exit code for flag/usage errors — see Assumptions #6):

  ```
  Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.
  ```

  Rationale (recorded, non-obvious): the fetch path already writes day-files in multi mode on every data command, so a combined `--sync --dry-run` preview-then-proceed would mutate the very files it just previewed — a lying dry-run. Fail-fast is the honest contract, and strict→loose is the non-breaking direction if combined support is ever wanted. Silently ignoring the flag is ruled out: a user who passed `--dry-run` must never get a surprise mutation.

### 2. `writeMetrics` becomes preview-capable (shared decision path)

`writeMetrics(metricsDir, user, machine, toolKey, entries)` (src/node/sync/sync.ts) gains an optional dry-run mode and returns a per-file decision report instead of `void`. Shape (final naming at plan time):

```ts
interface WriteDecision {
  filePath: string;              // absolute day-file path
  action: "write" | "skip";      // skip = never-shrink guard
  incomingCost: number;
  existingCost?: number;         // present when an existing parseable file was read
}
```

- The decision logic — path construction, `isShrinkingWrite` — is executed identically in live and dry-run mode (this is the "shares the real code path" requirement). Only `mkdirSync`/`writeFileSync` are gated on dry-run.
- Live callers (`fetchToolMerged`, `fetchToolMergedWithMachines` in cli.ts, and `fullSync`) keep their existing call shape and ignore the return value — no behavior change on any live path.

### 3. `fullSync` dry-run: report, don't mutate

`fullSync(config, tuHome)` (src/node/sync/sync.ts) gains a dry-run mode:

- Fetches all tools exactly as today (`fetchHistory` — read-only, cached; same data feeds live and preview, keeping the preview accurate).
- Calls `writeMetrics` in dry-run mode, collecting `WriteDecision[]` per tool.
- Computes the git half of the preview **locally, no network**: would-be writes from the reports, plus `git status --porcelain {user}/` (read-only) for already-dirty files, determine whether a commit would happen and with which message (`# {user}: update {date}`, same string as live). `pull --rebase` / `push` are reported as the operations that would follow — never executed or probed (backlog: "without touching the working tree, the metrics repo, or the network").
- Skips `syncMetrics` and `touchLastSync` entirely; returns the structured report (printing stays out of sync.ts, which today writes nothing to stdout).

### 4. `runSync` prints the preview to stdout

`runSync` (src/node/core/cli.ts) passes dry-run through, formats the report, and prints to **stdout**, exit 0. Config/mode guards (multi-mode required, metrics-dir guard) run unchanged before the preview — a dry-run in single mode fails exactly like a live `tu sync`. Illustrative output (exact format is plan-level; final line states that nothing was mutated):

```
$ tu sync --dry-run
Would write 2 day-file(s) under ~/.tu/metrics_repo/sahil/:
  2026/mbp/cc-2026-07-18.jsonl     $12.34  (update: $10.20 → $12.34)
  2026/mbp/codex-2026-07-18.jsonl  $3.21   (new)
Would skip 1 file (never-shrink guard):
  2026/mbp/cc-2026-06-01.jsonl     incoming $0.00 < existing $45.67
Would commit: "# sahil: update 2026-07-18", then pull --rebase origin main, then push
Dry run — nothing written, committed, or pushed.
```

### 5. Help, completions, README

- `FULL_HELP` (src/node/core/cli.ts): add a `--dry-run` line under Flags (e.g. `--dry-run            Preview sync without writing (tu sync only)`). `tu help-dump` derives from `FULL_HELP` at runtime, so the published help contract updates by construction — no help-dump.ts change.
- `completions.ts` (src/node/core/completions.ts): add `--dry-run` to all three shells — bash `long_flags`, zsh `long_flags` array + descriptive spec (`'--dry-run[preview sync without writing]'`), fish (`complete -c tu -l dry-run -d '...'`).
- `README.md` flags list (mirrors FULL_HELP, per the readme-extraction standard) and `docs/specs/usage.md` flags table / Sync Flow section get the matching one-line additions.

### 6. Tests

Co-located per constitution: `src/node/sync/__tests__/` and `src/node/core/__tests__/`.

- Dry-run `writeMetrics` returns correct decisions (new / update / never-shrink skip) and leaves the filesystem untouched.
- Live `writeMetrics` behavior byte-identical to today (report addition is non-behavioral).
- Dry-run `fullSync` performs no git operations, no `.last-sync` touch, no file writes.
- Flag parsing: `dryRunFlag` set, filtered from `filteredArgs`; misuse (`tu cc --dry-run`, `tu cc --sync --dry-run`) errors on stderr with exit 1.

## Affected Memory

- `sync/multi-machine`: (modify) document dry-run mode of `fullSync`/`writeMetrics` (decision report, no-mutation guarantee, git-preview-without-network) alongside the existing never-shrink requirements
- `cli/data-pipeline`: (modify) document the `--dry-run` global flag, its sync-only honoring, and the fail-fast misuse contract

## Impact

- `src/node/sync/sync.ts` — `writeMetrics` report + dry-run gate; `fullSync` dry-run mode (core of the change)
- `src/node/core/cli.ts` — `GlobalFlags`/`parseGlobalFlags`, misuse guard in `main()`, `runSync` preview formatting, `FULL_HELP`
- `src/node/core/completions.ts` — three shells
- `README.md`, `docs/specs/usage.md` — flag documentation
- `src/node/sync/__tests__/`, `src/node/core/__tests__/` — new coverage
- No new dependencies; no output-format change to any existing command (additive flag ⇒ minor version per Output Stability)

## Open Questions

*(none — the two decide-at-intake points from the backlog are resolved with evidence; see Origin and Assumptions)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Preview prints to stdout, exit 0 (`tu sync --dry-run`) | Backlog deferred this to intake; resolved by observing the standard's reference implementation (`shll uninstall --dry-run` → stdout, empty stderr) + principle №2: the preview is the data the caller asked for | S:60 R:70 A:90 D:80 |
| 2 | Confident | `--dry-run` honored only by `tu sync`; all other invocations (incl. `tu <cmd> --sync --dry-run`) fail fast with an actionable error | Fetch path writes day-files outside the sync boundary in multi mode, so a combined preview-then-proceed would mutate what it previewed; fail-fast is honest and strict→loose is the non-breaking direction | S:50 R:65 A:55 D:45 |
| 3 | Certain | No `--yes`/consent gate added | Backlog explicit: never-shrink guard makes sync additive — preview-for-safety, not destructive-consent | S:90 R:80 A:90 D:90 |
| 4 | Confident | `writeMetrics` gains optional dry-run + returns per-file decision report; live callers ignore the return value | Only design that shares the real decision path (`isShrinkingWrite` + path construction) per №5's accuracy requirement while leaving all live call sites behaviorally unchanged | S:70 R:75 A:80 D:70 |
| 5 | Certain | Git half of preview computed locally (would-be writes + `git status --porcelain`); pull/push reported, never probed — no network | Backlog explicit: "without touching the working tree, the metrics repo, or the network" | S:85 R:80 A:85 D:80 |
| 6 | Confident | Misuse error exits 1 (not the toolkit's exit-2 usage convention) | tu uses exit 1 uniformly today, including all existing flag-validation errors in `parseGlobalFlags` (cli.ts:708–770); adopting exit 2 for one flag would be inconsistent locally and a scheme change is out of scope | S:70 R:80 A:85 D:70 |
| 7 | Certain | Dry-run still fetches tool data via the unchanged `fetchHistory` path (cached, read-only) | An accurate preview requires the same inputs as the live run; fetching is non-mutating | S:75 R:85 A:90 D:85 |

7 assumptions (3 certain, 4 confident, 0 tentative, 0 unresolved).
