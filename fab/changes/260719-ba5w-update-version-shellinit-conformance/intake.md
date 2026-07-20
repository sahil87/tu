# Intake: shll update/version/shell-init Standards Conformance

**Change**: 260719-ba5w-update-version-shellinit-conformance
**Created**: 2026-07-20

## Origin

One-shot `/fab-new` invocation. Raw input:

> Bring this repo into conformance with the shll toolkit 'update', 'version', and 'shell-init' standards (docs/site/standards/update.md, version.md, and shell-init.md in the shll repo, or https://shll.ai/standards). Audit the update, --version, and shell-init subcommands against every MUST/SHOULD in all three standards, fix any gaps found, and add/update tests pinning the fixed behavior. If the audit finds the repo is already fully conformant with no code changes needed, skip /git-pr entirely — do not open an empty PR.

The audit was performed at intake time against the live standards (`shll standards update`, `shll standards version`, `shll standards shell-init`, shll v0.0.23+ as installed 2026-07-20) and **found non-conformances**, so the conditional "skip /git-pr if fully conformant" branch does NOT apply — this change proceeds through the normal pipeline. The full audit findings are recorded under What Changes.

This continues the conformance series: the constitution's `### Toolkit Standards` article (v1.1.0) binds CLI-surface changes to the published standards; prior audits covered `principles`, `help-dump`, `readme-extraction`, `skill` (260717-rdo3) and toolkit naming (260718-a3z0). The `update`, `version`, and `shell-init` standards are newer additions to `shll standards` and have not been audited before.

## Why

1. **Problem**: tu is one of the six roster tools (`wt`, `idea`, `tu`, `run-kit`, `hop`, `fab-kit`) that `shll update`, `shll version`, `shll doctor`, and `shll shell-init` compose over. Three newly published producer-facing standards define frozen contracts for `update`, `--version`, and `shell-init`, and tu currently violates several MUSTs (audit details in What Changes).
2. **Consequence if unfixed**:
   - `shll update`'s flag-discovery probe runs `tu update --help` — tu currently **executes a real update** on that invocation instead of printing help, so the probe both mutates state and never sees the literal `--skip-brew-update` substring, silently degrading every `shll update` run to redundant `brew update`s.
   - The 120s hard timeout on `brew upgrade` is the **exact incident** the standard's brew-safety clause documents (2026-07-19: a stalled GitHub API call inside `brew upgrade` exceeded a 120-second kill, corrupting the keg mid-swap → `zsh: permission denied: <tool>`).
   - `tu shell-init` with no argument prints usage to **stdout** and exits **0** — `shll shell-init` drops tools only on non-zero exit, so this usage text would sail verbatim into `eval "$(shll shell-init …)"` on every shell startup if shll ever invoked it without an argument.
3. **Approach**: minimal, surgical fixes to `src/node/core/cli.ts` scoped to exactly the audited MUST/SHOULD gaps, plus tests pinning each fixed contract (the standards' own "Verifying conformance" sections each ask for a pinning test). No redesign of the update/shell-init surfaces.

## What Changes

### Audit results (full MUST/SHOULD walk)

**`update` standard** (`shll standards update`):

| Clause | Status |
|--------|--------|
| MUST expose `update` subcommand, in-place upgrade, works standalone | ✅ conformant (`runUpdate` in `cli.ts`) |
| MUST advertise literal `--skip-brew-update` in `tu update --help` | ❌ **VIOLATION** — `tu update --help` dispatches to `runUpdate` and performs a real update; help is only detected when `--help`/`-h`/`help` is the *first* arg (`cli.ts` main(), ~line 1388) |
| MUST honor `--skip-brew-update` (skip internal `brew update`) | ✅ conformant (gate in `runUpdate`, pinned by `cli-skip-brew-update-flag.test.ts`) |
| MUST exit 0 on success incl. already-up-to-date | ✅ conformant (up-to-date path returns, exit 0) |
| MUST exit non-zero only on genuine failure | ✅ conformant (exit 1 on brew update/info/upgrade failure) |
| MUST NOT SIGKILL a package-manager subprocess mid-transaction | ✅ technically conformant (Node `execSync` default `killSignal` is SIGTERM) — but see next row |
| MUST NOT impose a short hard timeout on `brew upgrade` | ❌ **VIOLATION** — `execSync("brew upgrade tu", { stdio: "inherit", timeout: 120_000 })` is the standard's cited failure incident verbatim |
| SHOULD: any bound generous, SIGTERM + grace | ⚠️ gap — `brew update --quiet` capped at 30s (brew can legitimately block minutes on network → false failure, exit 1, false `shll update` failure); `brew info` capped at 10s |
| SHOULD self-update via brew only when brew-installed, `/Cellar/` gate, clear degrade message | ✅ conformant (`__cli_dirname.includes("/Cellar/tu/")` gate, friendly message, exit 0) |
| One name four places; `v{semver}` tags; rename → `formula_renames.json` | ✅ conformant (repo/roster/formula/binary all `tu`; releases tagged `v*` per `release.yml`) — no rename in this change |

**`version` standard** (`shll standards version`):

| Clause | Status |
|--------|--------|
| MUST support `--version`, exit 0, version on stdout | ✅ conformant |
| MUST respond within 2s, no network I/O | ✅ conformant (`PKG_VERSION` embedded at build time; printed before any fetch) |
| Version token on first non-empty line; RECOMMENDED `<tool> version vX.Y.Z` | ✅ conformant — prints exactly `tu version v{X.Y.Z}` as the only line |
| Binary name on PATH == tool name | ✅ conformant (`tu`) |
| "Keep (or add) a minimal test pinning the above" | ❌ **gap** — no test covers `--version` output shape/exit code |

**`shell-init` standard** (`shll standards shell-init`):

| Clause | Status |
|--------|--------|
| MUST emit eval-safe shell source on stdout, exit 0, for `zsh` and `bash` | ✅ conformant (`BASH_COMPLETION`/`ZSH_COMPLETION` emitted verbatim; fish additionally supported — out of standard scope, kept) |
| Diagnostics to stderr only | ✅ conformant on the success paths (stdout carries only the script) |
| On any failure exit non-zero | ✅ conformant for unknown shell (stderr + exit 2) |
| Missing/unsupported shell arg MUST exit non-zero (convention 2), usage on **stderr**, stdout **empty** | ❌ **VIOLATION** — `tu shell-init` with no argument prints `SHELL_INIT_USAGE` to **stdout** via `console.log` and exits **0** (`runShellInit`, `cli.ts` ~line 315). `completions.test.ts` ("runShellInit: no argument") currently pins this wrong behavior |
| "Keep (or add) a test that evals the output in a subshell and asserts a clean exit" | ❌ **gap** — `completions.test.ts` asserts string content only; nothing evals the emitted source |

### Fix 1: `tu update --help` prints help instead of running the update

In `main()`'s dispatch (`src/node/core/cli.ts`), before `cmd === "update"` invokes `runUpdate`, detect `--help`/`-h` among the update invocation's args and print `FULL_HELP` (which already contains the literal `--skip-brew-update` substring at its Flags section) to stdout, exit 0. Scoped to the `update` subcommand only — other surfaces' `--help` behavior (e.g. `tu cc --help` → exit 2 unknown-argument) is pinned by existing tests and is not governed by these three standards, so it stays untouched.

Concrete shape (dispatch site):

```ts
if (cmd === "update") {
  if (rawArgs.includes("--help") || rawArgs.includes("-h")) { console.log(FULL_HELP); return; }
  runUpdate(process.argv.includes("--skip-brew-update"));
  return;
}
```

(Note: `-h`/`--help` are not stripped by `parseGlobalFlags`, so checking `rawArgs` or `filteredArgs` is equivalent here; use whichever reads cleaner at the site.)

### Fix 2: remove the hard timeout on `brew upgrade`; make the other brew bounds generous

In `runUpdate`:

- `execSync("brew upgrade tu", { stdio: "inherit" })` — **timeout removed entirely**. The call is interactive (`stdio: "inherit"`), brew legitimately blocks for minutes, and the user can Ctrl-C. This is the standard's preferred posture ("MUST NOT impose a short hard timeout"; a bound is optional, not required).
- `brew update --quiet`: raise timeout 30_000 → **600_000** (10 min — sized for a network transfer per the SHOULD; Node's default SIGTERM kill already satisfies "terminate gracefully"). Kept bounded because this path is non-interactive (`stdio: "pipe"`) and a hang here would otherwise block silently with no output.
- `brew info --json=v2 tu`: raise timeout 10_000 → **60_000** (mostly-local command; generous headroom).

### Fix 3: `tu shell-init` with missing shell arg → usage on stderr, exit 2, empty stdout

In `runShellInit` (`src/node/core/cli.ts`): the `shell === undefined` branch changes from `console.log(SHELL_INIT_USAGE); return;` to writing `SHELL_INIT_USAGE` to **stderr** and `process.exit(EXIT_USAGE)` (= 2). Content of `SHELL_INIT_USAGE` is unchanged. The unknown-shell branch already conforms (stderr, exit 2) and is untouched.

Update the pinning test in `src/node/core/__tests__/completions.test.ts` ("runShellInit: no argument") to assert the new contract: stderr carries the usage, stdout is empty, exit code 2. Per the constitution's Test Integrity article, the standard is the spec and the test moves to match it.

### Fix 4: new conformance tests

New test file `src/node/core/__tests__/cli-version.test.ts` (subprocess pattern per `cli-exit-codes.test.ts`'s `runCli` helper — `spawnSync("npx", ["tsx", CLI, ...])`):

- `tu --version` exits 0, stdout's first non-empty line matches `^tu version v\d+(\.\d+)*` (both the `versionPrefixRE` shape and the token regex from the standard), stderr empty.
- Same for `-V` and `-v` aliases (same code path; one representative assertion each).

Extend `src/node/core/__tests__/completions.test.ts` (or a new `cli-shell-init.test.ts` if cleaner):

- **Eval test**: `bash -c 'eval "$(…)"'` over the emitted bash script asserts exit 0; same for zsh **guarded by availability** (skip with a note when `zsh` is not on PATH — CI runners may lack it). This is the standard's "evals the output in a subshell and asserts a clean exit" guard.
- Missing-arg contract test (subprocess): `tu shell-init` → exit 2, empty stdout, usage on stderr.

New/extended update test (subprocess): `tu update --help` exits 0, stdout contains the literal substring `--skip-brew-update`, and **no brew command runs** (the help short-circuit returns before `runUpdate`; assert via exit-fast behavior — the test must not depend on brew being installed). Also a source-level assertion that the `brew upgrade` call site carries no `timeout` option (pins Fix 2 the same way `cli-skip-brew-update-flag.test.ts` mirrors the gate — or simpler: read `cli.ts` source in the test and assert the `brew upgrade tu` execSync options carry no short timeout; choose the least brittle idiom at apply time).

### Out of scope

- `fish` shell-init support: kept as-is (standard scope is zsh/bash; fish is a superset, not a violation).
- `tu shell-init --help` / other subcommands' `--help` handling: not governed by these standards; existing behavior stays.
- Qualifying `brew upgrade tu` → `brew upgrade sahil87/tap/tu`: the producer standard does not require it; naming alignment already holds.
- README/docs/site updates: none of the three standards impose repo-doc obligations on the producer beyond the binary behavior.

## Affected Memory

- `build/toolchain.md`: (modify) extend the toolkit-standards conformance posture entry — record that the `update`, `version`, and `shell-init` standards are now audited/adopted, the violations found, and the brew-timeout safety posture (no hard timeout on `brew upgrade`)
- `cli/data-pipeline.md`: (modify) record the CLI-surface contract changes: `tu update --help` short-circuit, `shell-init` missing-arg exit-2/stderr contract

## Impact

- **Code**: `src/node/core/cli.ts` only (dispatch site + `runUpdate` + `runShellInit`) — ~15 lines changed.
- **Tests**: `src/node/core/__tests__/completions.test.ts` (one pinned behavior updated, eval tests added), new `cli-version.test.ts`, update-help test (new file or extension of `cli-skip-brew-update-flag.test.ts`).
- **Behavior/output stability**: `tu shell-init` (no arg) changes exit 0→2 and stdout→stderr — a deliberate breaking change mandated by the standard; per the constitution's Output Stability article this SHOULD ride a minor version bump, which the release-PR flow assigns at release time (no version file is edited in this change).
- **No dependency, config, or docs/site changes.**

## Open Questions

*(none — the input was fully specified and the standards deterministically answer each decision; zero Unresolved assumptions)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Fix scope = the 3 MUST violations + 2 test gaps found by the audit; `--version` behavior itself unchanged | Direct clause-by-clause audit against the live standard text (`shll standards …`); standards deterministically classify each surface | S:90 R:85 A:95 D:90 |
| 2 | Confident | `tu update --help` fix is scoped to the `update` dispatch only (prints `FULL_HELP`, exit 0); no global `--help`-anywhere handling | Standard mandates only `<tool> update --help`; global handling would disturb pinned exit-2 behavior on data commands — smallest conformant diff wins | S:75 R:80 A:80 D:65 |
| 3 | Certain | Remove the `brew upgrade` timeout entirely rather than raising it | Standard: "MUST NOT impose a short hard timeout"; a bound is optional and the call is interactive (`stdio: inherit`) — removal is the cited tools' (hop/fab-kit) posture | S:80 R:90 A:85 D:75 |
| 4 | Confident | Raise (not remove) the non-interactive brew bounds: `brew update` 30s→600s, `brew info` 10s→60s | SHOULD clause asks for generous bounds "sized for a network transfer"; keeping a bound on piped (non-interactive) calls avoids silent hangs; SIGTERM default satisfies graceful termination | S:70 R:85 A:75 D:60 |
| 5 | Certain | `shell-init` missing-arg: usage → stderr, exit `EXIT_USAGE` (2), stdout empty; existing wrong-pinning test updated to the standard | Standard MUST with explicit convention (exit 2); constitution Test Integrity: tests conform to spec, standards are the spec | S:90 R:85 A:95 D:90 |
| 6 | Tentative | Eval-in-subshell test mechanics: bash eval always asserted; zsh eval skipped-with-note when `zsh` absent from PATH; update-help test avoids any brew dependency | Standard asks for the eval guard but not its mechanics; CI runner shell availability is environment-dependent — skip-guard is the safe default <!-- assumed: zsh availability guard in eval tests — CI runners may lack zsh; skip-with-note keeps the suite green while still guarding where zsh exists --> | S:55 R:90 A:60 D:50 |
| 7 | Certain | Keep fish shell-init support unchanged | Standard scope is zsh/bash; fish is additive and violates nothing | S:85 R:90 A:95 D:90 |
| 8 | Confident | No version bump in this change; the release-PR flow assigns the minor bump the Output Stability article asks for | Versioning is owned by `release.yml`'s release-PR path per build/toolchain memory; editing versions mid-change would fight that flow | S:65 R:75 A:80 D:70 |

8 assumptions (4 certain, 3 confident, 1 tentative, 0 unresolved).
