# Intake: Add `--skip-brew-update` flag to `update` command

**Change**: 260531-e96v-skip-brew-update-flag
**Created**: 2026-05-31
**Status**: Draft

## Origin

This change was initiated one-shot (no prior `/fab-discuss`) from a detailed cross-toolkit
contract. The raw user input:

> Add a boolean `--skip-brew-update` flag to the `update` command. CONTRACT (cross-toolkit,
> identical in 6 tools): flag name EXACTLY `--skip-brew-update`. When set, skip ONLY the internal
> `brew update --quiet` tap-metadata refresh. Everything else unchanged: the `brew info` version
> check, the up-to-date short-circuit, and `brew upgrade`. Default (absent) = current behavior
> exactly preserved. THIS REPO (tu, TypeScript/Node, SPECIAL CASE — no flag-parsing library):
> file `src/node/core/cli.ts`. `update` is dispatched by a bare string switch `if (cmd === "update")`
> ~L1138. `runUpdate()` is ~L273; inside it `execSync("brew update --quiet", ...)` ~L283 is the line
> to gate, `execSync("brew info ...")` ~L291 and `execSync("brew upgrade tu")` ~L311 must stay intact.
> Detect `--skip-brew-update` from `process.argv` (or the args array the dispatcher already holds),
> pass a boolean into `runUpdate()`, gate only the `brew update --quiet` call. Update the `tu update`
> help text (~L72) if it documents flags. Match tu existing subprocess convention (`execSync`). Add a
> test following tu existing test pattern in `src/node/core/__tests__/` asserting `--skip-brew-update`
> omits `brew update` but still runs `brew upgrade`.

The contract is unusually prescriptive — exact flag name, exact file, exact lines, and exact behavior
boundaries are specified. This is one of six identical implementations across a toolkit; tu is flagged
as the SPECIAL CASE because it has no flag-parsing library (flags are detected via `rawArgs.includes(...)`).

## Why

1. **Problem**: `tu update` always runs `brew update --quiet` to refresh Homebrew tap metadata before
   checking for a new `tu` version. In automated/CI contexts, or when a caller has *just* run
   `brew update` themselves (e.g. a wrapper script updating six toolkit binaries in sequence), this
   internal refresh is redundant — it re-fetches all tap metadata, which is slow and hits the network
   on every invocation.

2. **Consequence if not fixed**: Callers orchestrating multiple toolkit `update` commands pay the
   `brew update` cost six times over, with no way to opt out. The network round-trip also makes
   `tu update` fail (exit 1) when offline even if the desired version is already cached locally —
   the up-to-date short-circuit never gets a chance to run.

3. **Why this approach**: A single boolean flag that gates *only* the internal refresh is the minimal,
   composable solution. The cross-toolkit contract mandates the exact flag name (`--skip-brew-update`)
   and exact semantics so a single orchestrator can pass it uniformly to all six tools. tu must match
   that contract byte-for-byte despite its hand-rolled flag parsing.

## What Changes

### 1. `runUpdate()` gains a parameter to gate the internal refresh

`runUpdate()` in `src/node/core/cli.ts` (~L273) currently takes no arguments and unconditionally runs:

```ts
try {
  execSync("brew update --quiet", { stdio: "pipe", timeout: 30_000 });
} catch {
  console.error("Error: could not check for updates (brew update failed). Check your network connection.");
  process.exit(1);
}
```

It SHALL accept a boolean parameter (default `false`) that, when `true`, skips this `brew update --quiet`
block entirely. The signature becomes `runUpdate(skipBrewUpdate = false)`. When the flag is set, execution
proceeds directly to the `brew info --json=v2 tu` version check.

Everything else in `runUpdate()` is UNCHANGED:
- The Homebrew-install guard (`__cli_dirname.includes("/Cellar/tu/")`) at the top — unchanged.
- `execSync("brew info --json=v2 tu", ...)` (~L291) — unchanged, always runs.
- The up-to-date short-circuit (`if (latest === PKG_VERSION) { ... return; }`, ~L303) — unchanged.
- `execSync("brew upgrade tu", ...)` (~L311) — unchanged, always runs when an upgrade is needed.

### 2. Dispatcher detects the flag and passes it in

The dispatch site (~L1138) is currently:

```ts
if (cmd === "update") { runUpdate(); return; }
```

It SHALL detect `--skip-brew-update` and pass the boolean into `runUpdate()`. tu's established convention
for boolean flags is membership testing against the raw argv array (e.g. `rawArgs.includes("--sync")` in
`parseGlobalFlags`, L518). The detection SHALL follow that same idiom. Because `--skip-brew-update` is a
command-specific flag (not a global data-display flag) and `update` ignores positional arguments, detecting
it from the raw process arguments is the lowest-blast-radius approach — it does not touch `parseGlobalFlags`
or the `GlobalFlags` interface, so no data-command flag handling is perturbed.

Resulting dispatch:

```ts
if (cmd === "update") { runUpdate(process.argv.includes("--skip-brew-update")); return; }
```

(`process.argv` is the raw argv; the flag is matched there exactly as `--sync`/`--fresh` are matched against
`rawArgs`. The dispatcher already holds `filteredArgs`, but global-flag filtering does not strip
`--skip-brew-update`, so it would also survive in `filteredArgs` — either source works. `process.argv` is the
form the contract names first and is unambiguous.)

### 3. Help text documents the new flag

`FULL_HELP` (the `Flags:` block, ~L77–88) documents every flag. A new line SHALL be added describing
`--skip-brew-update`. The existing entries follow the column-aligned format:

```
  --by-machine         Show per-machine cost breakdown (data commands only)
```

New entry (aligned to the same column, scoped note clarifying it applies to `update`):

```
  --skip-brew-update   Skip 'brew update' tap refresh during 'tu update'
```

### 4. Test asserting the gate

A new test file `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts` SHALL follow the existing
pattern in `cli-fresh-flag.test.ts` / `cli-sync-flag.test.ts`. It MUST assert:
- When `--skip-brew-update` is set, `brew update` is NOT invoked.
- `brew upgrade` IS still invoked (the upgrade path is preserved).
- Detection idiom: `process.argv.includes("--skip-brew-update")` (or the equivalent membership test) yields
  the correct boolean.

**Resolved (user decision):** The test SHALL mirror the command-selection logic inline, following the
`cli-sync-flag.test.ts` precedent (which mirrors the `--sync` guard inline rather than spawning subprocesses).
`runUpdate` calls a static `execSync` import directly, which ESM/tsx cannot cleanly intercept without
refactoring `runUpdate` for dependency injection — and that refactor is outside the contract's minimal
surface. So instead of mocking `execSync`, the test reproduces the gate's command sequence as a small pure
helper and asserts:
- `skip = true` → command sequence contains `brew upgrade tu` but NOT any `brew update ...` command.
- `skip = false` → command sequence contains both `brew update --quiet` and `brew upgrade tu`.

This requires NO production refactor beyond the `runUpdate` boolean param + dispatch detection + help line.
It satisfies the contract's "omits `brew update` but still runs `brew upgrade`" assertion exactly, in tu's
established test style.

## Affected Memory

- `cli/data-pipeline`: (modify) The `update` command gains a `--skip-brew-update` flag that gates the
  internal `brew update --quiet` refresh. This is a user-facing CLI grammar change to a non-data command,
  so the CLI memory should note the flag and its narrow scope (refresh only; version check and upgrade
  unaffected).

## Impact

- **Code**: `src/node/core/cli.ts` — `runUpdate()` signature (+1 boolean param), the `update` dispatch
  line, and the `FULL_HELP` constant. Single file.
- **Tests**: one new file under `src/node/core/__tests__/`.
- **APIs/contracts**: extends the public CLI surface (`tu update --skip-brew-update`). Per the constitution's
  Output Stability rule, this is additive (no change to default behavior) — a patch/minor bump suffices;
  no breaking change.
- **Dependencies**: none. Uses the existing `execSync` from `node:child_process`.
- **Build**: `npm run build` produces the single ESM bundle; must pass. Relevant test must pass before PR.
- **Constitution check**: Single-Purpose CLI ✓ (serves the core update mechanism), Single-Bundle ✓
  (no new deps), no new test framework ✓ (Node built-in runner, co-located `__tests__/`).

## Open Questions

None. The contract fully specifies flag name, scope, file, lines, and the default-preserves-behavior
requirement. The one spec-stage choice — the test mechanism — was resolved by the user: mirror the
command-selection logic inline (per `cli-sync-flag.test.ts`), no `execSync` mocking, no production refactor.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Flag name is EXACTLY `--skip-brew-update` | Contract mandates the exact string, identical across 6 tools; no latitude | S:100 R:90 A:100 D:100 |
| 2 | Certain | Gate ONLY `execSync("brew update --quiet", ...)` (~L283); leave `brew info` (~L291), the up-to-date short-circuit (~L303), and `brew upgrade tu` (~L311) intact | Contract enumerates exactly which line to gate and which must stay; verified against current cli.ts | S:100 R:85 A:100 D:100 |
| 3 | Certain | Default (flag absent) preserves current behavior exactly | Contract states default = current behavior exactly preserved; boolean param defaults to false | S:100 R:90 A:100 D:100 |
| 4 | Confident | `runUpdate()` gains a boolean param `skipBrewUpdate = false`; dispatcher passes `process.argv.includes("--skip-brew-update")` | Contract says "pass a boolean into runUpdate()" and "detect from process.argv"; matches tu's `rawArgs.includes(...)` idiom (parseGlobalFlags L518). Default param keeps existing callers/tests valid | S:90 R:80 A:90 D:85 |
| 5 | Confident | Detect via raw-argv membership test, NOT by extending `parseGlobalFlags`/`GlobalFlags` | `--skip-brew-update` is command-specific (update only), not a data-display flag; `update` ignores positionals. Keeping it out of parseGlobalFlags is lowest blast radius and matches the contract's "from process.argv" wording | S:85 R:75 A:85 D:80 |
| 6 | Confident | Add one `FULL_HELP` flag line; scope note ties it to `tu update` | Contract says update help text "if it documents flags" — it does (Flags: block L77). Aligns to existing column format | S:90 R:90 A:90 D:85 |
| 7 | Confident | New test file `cli-skip-brew-update-flag.test.ts` co-located in `src/node/core/__tests__/`, Node built-in runner | Contract names the directory and reference files; constitution mandates co-located `__tests__/` + Node runner. Naming follows `cli-{flag}-flag.test.ts` convention | S:90 R:85 A:95 D:80 |
| 8 | Confident | Test mirrors the gate's command-selection logic inline (per `cli-sync-flag.test.ts`); no `execSync` mock, no `runUpdate` refactor. Asserts skip=true omits `brew update` but keeps `brew upgrade tu`; skip=false includes both | Resolved — user chose inline-mirror over injectable-runner refactor. Matches existing test precedent and keeps production change to the minimal contract surface (param + dispatch + help) | S:90 R:80 A:90 D:90 |

8 assumptions (3 certain, 5 confident, 0 tentative, 0 unresolved).
