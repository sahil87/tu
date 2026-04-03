# Intake: Add -v Version Shorthand

**Change**: 260401-kuuh-add-v-version-shorthand
**Created**: 2026-04-01
**Status**: Draft

## Origin

> User request: "Make tu -v the same as --version"

One-shot instruction. No prior discussion.

## Why

CLI tools conventionally support `-v` as a shorthand for `--version` (e.g., `node -v`, `git --version`, `docker -v`). Currently `tu` uses `-V` (capital V) as the shorthand, which is unexpected. Users who type `tu -v` expecting version output get no match — the flag is silently ignored and the CLI proceeds to its default data-display behavior. Adding `-v` aligns with common CLI conventions and reduces friction.

## What Changes

### Add `-v` to version flag check (`src/node/core/cli.ts`)

Line 1014 currently checks:

```ts
if (rawArgs.includes("--version") || rawArgs.includes("-V")) {
```

Add `|| rawArgs.includes("-v")` so that `-v` (lowercase) also triggers version output:

```ts
if (rawArgs.includes("--version") || rawArgs.includes("-V") || rawArgs.includes("-v")) {
```

This prints `PKG_VERSION` and returns, same as the existing `--version` / `-V` behavior.

No other code changes are needed — `-v` is not used as a flag anywhere else in the CLI (`parseGlobalFlags` does not reference it, and no subcommand claims it).

## Affected Memory

- `cli/data-pipeline`: (modify) Document that `-v` is now a recognized version flag alongside `--version` and `-V`

## Impact

- **`src/node/core/cli.ts`** — single condition change on line 1014
- **No breaking changes** — `-v` was previously unrecognized and silently ignored
- **No new dependencies**

## Open Questions

None.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Keep `-V` as a valid alias | Backward compatibility — existing users may rely on `-V` | S:90 R:95 A:95 D:95 |
| 2 | Certain | No conflict with other flags | Verified: `-v` is unused in `parseGlobalFlags` and all subcommands | S:95 R:90 A:95 D:95 |
| 3 | Certain | Same output as `--version` (prints `PKG_VERSION` and exits) | Direct requirement from user input — "the same as --version" | S:95 R:95 A:95 D:95 |

3 assumptions (3 certain, 0 confident, 0 tentative, 0 unresolved).
