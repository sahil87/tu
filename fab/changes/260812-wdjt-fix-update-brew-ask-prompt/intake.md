# Intake: Fix `tu update` hang on Homebrew 6 ask-mode prompt

**Change**: 260812-wdjt-fix-update-brew-ask-prompt
**Created**: 2026-08-12

## Origin

Promptless dispatch (`/fab-proceed`-style create-intake, `{questioning-mode} = promptless-defer`) from a live debugging conversation. The user diagnosed a `tu update` hang empirically on this machine (Homebrew 6.0.17) and arrived at a specific, verified fix before intake creation.

> **Problem**: `tu update` — unlike every other shll toolkit tool — hangs mid-update waiting for a y/n confirmation. Root cause: Homebrew 6 made "ask mode" the DEFAULT for `brew upgrade`. tu's `runUpdate` runs `execSync("brew upgrade tu", { stdio: "inherit" })`, so with a real terminal both stdin and stdout are TTYs and Homebrew's `Do you want to proceed with the upgrade? [y/n]` prompt fires and blocks.
>
> **Decision**: pass `HOMEBREW_NO_ASK: "1"` in the child environment for the `brew upgrade tu` call. Keep `stdio: "inherit"`.

Key decisions were made and verified in the conversation (see Assumptions); this intake was generated without asking further questions per the promptless-defer contract.

## Why

1. **The pain point**: `tu update` hangs mid-update on Homebrew 6 machines. Homebrew 6 made "ask mode" the default for `brew upgrade` (`Library/Homebrew/cmd/upgrade.rb`: "Do not ask for confirmation before downloading and upgrading. Ask mode is the default." — disabled via `--no-ask`/`--yes`/`-y` flags or the `HOMEBREW_NO_ASK=1` env var). The prompt is Homebrew's `Do you want to proceed with the upgrade? [y/n]` from `Library/Homebrew/ask.rb`, and it fires only when BOTH stdin and stdout are TTYs (`return false if !$stdin.tty? || !$stdout.tty?`). tu's `runUpdate` (`src/node/core/cli.ts`, currently line ~386) runs `execSync("brew upgrade tu", { stdio: "inherit" })` — deliberately interactive per the toolkit `update` standard's brew-safety clause (no timeout, user can Ctrl-C). With inherited stdio in a real terminal, both fds are TTYs, so Homebrew's prompt fires and blocks. This was verified empirically on this machine (Homebrew 6.0.17): `brew reinstall tu </dev/null` printed the ask-mode preview (`==> Would reinstall 1 formula:`) but never prompted and completed with exit 0 — confirming the TTY-gated prompt is exactly what blocks the interactive call.

2. **The consequence if unfixed**: every `tu update` on a Homebrew 6 machine stalls indefinitely at an invisible-in-scripts y/n prompt. Other shll roster tools run their brew calls with piped/captured output, so Homebrew's TTY check silently skips the prompt — they're immune by accident. tu is the only toolkit tool that hangs, violating the toolkit-wide expectation that `update` runs to completion unattended.

3. **Why this approach**: the `HOMEBREW_NO_ASK=1` env var is the version-proof way to disable ask mode. The alternative — passing `--no-ask`/`--yes`/`-y` to `brew upgrade` — was rejected because Homebrew < 6 doesn't know the flag and would error, while an unrecognized `HOMEBREW_NO_ASK` env var is harmlessly ignored by older Homebrew versions.

## What Changes

### `runUpdate` in `src/node/core/cli.ts`: suppress Homebrew ask mode on the upgrade call

The single functional change. The current call (line ~386):

```ts
try {
    // NO timeout here (toolkit `update` standard MUST NOT): killing brew
    // mid-transaction corrupts the keg mid-swap. The call is interactive
    // (stdio: "inherit") — the user can Ctrl-C a genuinely stuck upgrade.
    execSync("brew upgrade tu", { stdio: "inherit" });
} catch {
    console.error("Error: brew upgrade failed.");
    process.exit(1);
}
```

becomes:

```ts
execSync("brew upgrade tu", { stdio: "inherit", env: { ...process.env, HOMEBREW_NO_ASK: "1" } });
```

- `HOMEBREW_NO_ASK: "1"` disables Homebrew 6's default ask mode, so `brew upgrade tu` never blocks on the `Do you want to proceed with the upgrade? [y/n]` prompt.
- `stdio: "inherit"` is KEPT — upgrade progress stays visible on the user's terminal and Ctrl-C still works (toolkit `update` standard brew-safety posture).
- NO `timeout` option is added (the standard's `MUST NOT impose a short hard timeout on brew upgrade` clause — killing brew mid-keg-swap corrupts the install; the cited 2026-07-19 incident).
- The spread of `process.env` preserves the full inherited environment (PATH, HOMEBREW_* user settings) — only the one variable is added.
- On Homebrew < 6, the unrecognized env var is harmlessly ignored; behavior is unchanged there.
- The explanatory comment above the call should be extended to note why `HOMEBREW_NO_ASK` is set (Homebrew 6 ask-mode default; env var over `--no-ask` flag for cross-version safety).

### Everything else in `runUpdate` stays as-is

- `brew update --quiet` (piped, 600s timeout) — unchanged, out of scope.
- `brew info --json=v2 tu` (piped, 60s timeout) — unchanged, out of scope.
- The `--skip-brew-update` contract is preserved untouched: the literal substring stays in `tu update --help` output, and the flag keeps skipping ONLY the internal `brew update`.
- Exit-code contract unchanged: 0 on success including already-up-to-date and non-Homebrew installs; 1 only on genuine brew failure.

### Test coverage

Per project review rules (`fab/project/code-review.md`: CLI behavior changes SHOULD include test coverage), add coverage for the new posture. Existing update-related tests live co-located in `src/node/core/__tests__/` (`cli-update-help.test.ts`, `cli-skip-brew-update-flag.test.ts`) and run via `npx tsx --test`. Note the established constraint documented in both files: `runUpdate` calls a statically-imported `execSync` that ESM/tsx cannot intercept, so those tests pin brew-call posture either at the source level (the no-timeout pin in `cli-update-help.test.ts`) or via a mirrored pure helper (`cli-skip-brew-update-flag.test.ts`). The `HOMEBREW_NO_ASK` posture should be pinned following the same precedent (e.g., a source-level assertion that the `brew upgrade tu` call site carries `HOMEBREW_NO_ASK: "1"` in its `env` and still carries `stdio: "inherit"` with no `timeout`).

### Explicit non-goals

- Fixing the toolkit `update` STANDARD itself (adding a "brew mutations must be non-interactive" clause) lives in the sahil87/shll repo and is explicitly OUT of scope for this tu change — this change fixes tu's `runUpdate` only.
- No new CLI flags, no help-text changes beyond none-at-all (the `--skip-brew-update` substring contract is a preservation constraint, not a change).

### Accepted behavioral trade-off

With the prompt suppressed, `tu update` upgrades without confirmation — which is what `tu update` already promises (the user asked for the update by running it) and matches the other roster tools' behavior.

## Affected Memory

- `cli/data-pipeline.md`: (modify) the `tu update` requirement pinning the interactive `brew upgrade tu` call (`stdio: "inherit"`, no timeout) gains the `HOMEBREW_NO_ASK: "1"` child-env clause and its Homebrew-6 ask-mode rationale
- `build/toolchain.md`: (modify) the "Toolkit `update` conformance" section's brew-timeout safety posture entry gains the ask-mode suppression posture (env var over `--no-ask` flag, cross-version rationale)

## Impact

- **Code**: one call site in `src/node/core/cli.ts` (`runUpdate`, line ~386) — add `env: { ...process.env, HOMEBREW_NO_ASK: "1" }` to the existing `execSync` options; extend the adjacent comment.
- **Tests**: `src/node/core/__tests__/` — extend/add a co-located test pinning the new posture alongside the existing no-timeout pin.
- **Docs/memory**: hydrate updates to `cli/data-pipeline.md` and `build/toolchain.md` (both currently document the exact `execSync("brew upgrade tu", { stdio: "inherit" })` form).
- **Toolkit standards**: conformant change — the `update` standard's brew-safety clause (no timeout, graceful termination) is preserved; the `--skip-brew-update` and exit-code contracts are untouched. The constitution's Toolkit Standards article requires checking CLI-surface changes against `shll standards update`; the constraints above were extracted from it in the originating conversation.
- **Runtime behavior**: only on Homebrew >= 6 with a real TTY does behavior change (no more blocking prompt); Homebrew < 6 and non-TTY contexts are byte-identical.
- **Versioning**: patch-level fix — no CLI output-format contract changes (the prompt was Homebrew's, not tu's).

## Open Questions

- (none — the originating conversation resolved the root cause, the fix, the rejected alternative, and all binding constraints)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Suppress ask mode via `HOMEBREW_NO_ASK: "1"` in the child env, not the `--no-ask`/`--yes`/`-y` flag | Discussed — user chose the env var; flag errors on Homebrew < 6 while the env var is harmlessly ignored (version-proof) | S:95 R:85 A:95 D:95 |
| 2 | Certain | Keep `stdio: "inherit"` and NO timeout on `brew upgrade tu` | Toolkit `update` standard brew-safety clause (MUST NOT hard-timeout brew); progress visibility + Ctrl-C preserved — explicit constraint in discussion | S:95 R:80 A:95 D:95 |
| 3 | Certain | Scope = tu's `runUpdate` only; the toolkit `update` standard amendment lives in sahil87/shll and is out of scope | Explicit scope boundary stated in discussion | S:95 R:90 A:95 D:95 |
| 4 | Certain | Accept upgrade-without-confirmation behavior | Discussed and accepted — `tu update` already promises the update; matches other roster tools | S:90 R:75 A:90 D:90 |
| 5 | Certain | `brew update --quiet` and `brew info --json=v2 tu` calls stay byte-identical; `--skip-brew-update` and exit-code contracts preserved | Explicitly declared fine-as-is/out of scope in discussion | S:95 R:90 A:95 D:90 |
| 6 | Confident | Pin the new posture with a source-level test following the existing precedent (statically-imported `execSync` is not interceptable under ESM/tsx) | Strong codebase precedent in `cli-update-help.test.ts` (no-timeout source pin) and `cli-skip-brew-update-flag.test.ts` (mirrored helper); exact mechanism left to apply <!-- assumed: source-level pin over execSync-injection refactor — matches documented test constraint in existing __tests__ files --> | S:70 R:85 A:80 D:60 |
| 7 | Certain | Memory hydrate targets are `cli/data-pipeline.md` and `build/toolchain.md` | Both files verified to pin the exact current `execSync("brew upgrade tu", { stdio: "inherit" })` form (260719-ba5w entries) | S:75 R:90 A:85 D:80 |
| 8 | Confident | Ship as a patch-level fix — no minor bump under the constitution's Output Stability rule | The suppressed prompt is Homebrew 6's, not part of tu's stable output contract; tu's own output format is unchanged | S:60 R:90 A:70 D:65 |

8 assumptions (6 certain, 2 confident, 0 tentative, 0 unresolved).
