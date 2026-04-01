# Intake: Add Self-Update Command

**Change**: 260307-9mfs-add-update-command
**Created**: 2026-03-07
**Status**: Draft

## Origin

> Add a `tu update` self-update command that shells out to `brew update && brew upgrade tu`. Detect if installed via Homebrew (check if pkgDir contains /Cellar/tu/), handle non-brew installs gracefully, show current vs latest version, and handle already-up-to-date case.

Conversational input preceded by exploration of the codebase and a plan-mode design session. Key decisions were made during discussion about implementation approach, Homebrew detection, and scope boundaries.

## Why

Currently updating tu requires remembering to run `brew update && brew upgrade tu` manually. There's no way to update from the CLI itself. Adding `tu update` makes this a single command, consistent with how other CLI tools handle self-updates.

Without this, users must context-switch to remember the Homebrew update workflow, and there's no feedback about whether they're running the latest version.

## What Changes

### 1. Add `runUpdate()` function in `src/node/core/cli.ts`

Place near other `run*` functions (after `runStatus`). The function:

1. **Detect Homebrew install**: Check if `_pkgDir` (already computed at top of cli.ts, line ~26-31) contains `/Cellar/tu/`. When tu is installed via Homebrew, the resolved path is something like `/opt/homebrew/Cellar/tu/0.2.6/libexec/lib/node_modules/tu/dist/tu.mjs`.

2. **Non-Homebrew path**: If not brew-installed, print a helpful message and return (exit 0, not crash):
   ```
   tu v0.2.6 was not installed via Homebrew.
   Update manually, or reinstall with: brew install wvrdz/tap/tu
   ```

3. **Homebrew update flow**:
   - Show `Current version: v{PKG_VERSION}`
   - Run `brew update --quiet` (refreshes tap, `stdio: "pipe"`, 30s timeout)
   - Get latest version via `brew info --json=v2 tu` (parse `formulae[0].versions.stable`, 10s timeout)
   - If same as `PKG_VERSION`: print `Already up to date (v{version}).` and return
   - If different: print `Updating v{current} → v{latest}...`, run `brew upgrade tu` with `stdio: "inherit"` (user sees build progress, 120s timeout)
   - Print `Updated to v{latest}.`

4. **Error handling** (per constitution principle II — Graceful Degradation):
   - `brew update` failure → `"Error: could not check for updates (brew update failed). Check your network connection."` + exit 1
   - `brew info` failure → `"Error: could not determine latest version."` + exit 1
   - `brew upgrade` failure → `"Error: brew upgrade failed."` + exit 1

Uses `execSync` (already imported in cli.ts) for all brew commands.

### 2. Wire command dispatch

Add to the non-data command if-chain in `main()` (after `if (cmd === "status")`):

```typescript
if (cmd === "update") { runUpdate(); return; }
```

### 3. Update `FULL_HELP` string

Add to the Setup section:

```
  tu update            Update tu to latest version
```

### 4. Update help test

Add assertion to `src/node/core/__tests__/cli-help.test.ts` in the "contains Setup section" test:

```typescript
assert.ok(FULL_HELP.includes("tu update"));
```

## Affected Memory

- `cli/data-pipeline`: (modify) Add `tu update` to CLI grammar non-data commands

## Impact

- **`src/node/core/cli.ts`**: Add `runUpdate()` (~30 lines), dispatch line, update `FULL_HELP`
- **`src/node/core/__tests__/cli-help.test.ts`**: Add one assertion
- No changes to config, sync, fetcher, or formatter modules

## Open Questions

- None — approach was fully designed in prior discussion.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Shell out to `brew update && brew upgrade tu` | Discussed — simplest approach for a Homebrew-distributed personal tool | S:95 R:85 A:90 D:95 |
| 2 | Certain | Detect Homebrew via `_pkgDir.includes("/Cellar/tu/")` | Discussed — reliable detection since Homebrew always installs to Cellar path | S:90 R:85 A:90 D:90 |
| 3 | Certain | Non-brew installs get a message and exit 0 (no crash) | Discussed — per constitution principle II (Graceful Degradation) | S:90 R:90 A:95 D:95 |
| 4 | Certain | Use `brew info --json=v2 tu` for version comparison | Discussed — reliable JSON output, no fragile text parsing | S:85 R:90 A:90 D:90 |
| 5 | Certain | No npm fallback auto-update | Discussed — primary distribution is Homebrew; keep it simple | S:85 R:90 A:85 D:90 |
| 6 | Certain | No `tu status` integration for update checking | Discussed — would add network latency, violates Fast Startup principle | S:80 R:90 A:90 D:90 |
| 7 | Certain | Add `tu update` to FULL_HELP Setup section only | Discussed — not a daily command, doesn't need SHORT_USAGE | S:80 R:95 A:85 D:90 |
| 8 | Confident | No new test file for `runUpdate` itself | Function shells out to brew — unit testing requires mocking execSync for little value; help text test covers integration | S:60 R:90 A:80 D:80 |
| 9 | Confident | `brew upgrade tu` uses `stdio: "inherit"` | User should see build progress (npm install + esbuild takes a few seconds) | S:50 R:90 A:85 D:85 |

9 assumptions (7 certain, 2 confident, 0 tentative, 0 unresolved).
