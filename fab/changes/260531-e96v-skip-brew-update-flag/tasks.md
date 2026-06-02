# Tasks: Add `--skip-brew-update` flag to `update` command

**Change**: 260531-e96v-skip-brew-update-flag
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

<!-- No scaffolding or dependencies needed — single existing file plus one new co-located test. -->

(none)

## Phase 2: Core Implementation

- [x] T001 In `src/node/core/cli.ts`, change `runUpdate()` (~L273) to accept a single boolean parameter with a default: `export function runUpdate(skipBrewUpdate = false): void`. Wrap ONLY the `execSync("brew update --quiet", { stdio: "pipe", timeout: 30_000 })` block (~L283, including its `try/catch` that errors + `process.exit(1)`) so it runs only when `!skipBrewUpdate`. Leave `brew info --json=v2 tu` (~L291), the `latest === PKG_VERSION` up-to-date short-circuit (~L303), and `brew upgrade tu` (~L311) completely unchanged.

- [x] T002 In `src/node/core/cli.ts`, update the `update` dispatch line (~L1138) from `if (cmd === "update") { runUpdate(); return; }` to detect the flag via tu's raw-argv membership idiom and pass the boolean in: `if (cmd === "update") { runUpdate(process.argv.includes("--skip-brew-update")); return; }`. Do NOT add the flag to `parseGlobalFlags` or the `GlobalFlags` interface.

## Phase 3: Integration & Edge Cases

- [x] T003 In `src/node/core/cli.ts`, add a `--skip-brew-update` entry to the `FULL_HELP` `Flags:` block (~L77–88), aligned to the existing column format (match the alignment of `--by-machine`). Suggested line: `  --skip-brew-update   Skip 'brew update' tap refresh during 'tu update'`.

## Phase 4: Polish

- [x] T004 [P] Create `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts` following the `cli-sync-flag.test.ts` pattern (Node built-in test runner: `import { describe, it } from "node:test"` + `import assert from "node:assert/strict"`). Mirror the gate's command-selection logic inline (do NOT mock `execSync`, do NOT spawn `brew`). Assert: (a) with `skipBrewUpdate = true`, the planned command sequence contains `brew upgrade tu` but NO `brew update ...` command; (b) with `skipBrewUpdate = false`, the sequence contains both `brew update --quiet` and `brew upgrade tu`; (c) the raw-argv membership test `["update","--skip-brew-update"].includes("--skip-brew-update")` is `true` and absent → `false`.

- [x] T005 Build and verify: run `npm run build` (must succeed — single ESM bundle, strict TS) and run the new test via the project's Node test-runner convention (e.g. `npx tsx --test src/node/core/__tests__/cli-skip-brew-update-flag.test.ts`). Both must pass before the change is considered apply-complete. Also run the two sibling flag tests (`cli-fresh-flag.test.ts`, `cli-sync-flag.test.ts`) to confirm no regression in flag handling.

---

## Execution Order

- T001 → T002 → T003 are edits to the same file (`cli.ts`); do them in sequence to avoid overlapping edits. T002 depends conceptually on T001's new signature.
- T004 (new test file) is independent of the `cli.ts` edits and may be written in parallel `[P]`, but its assertions mirror the gate logic from T001 — author it consistent with the final T001 behavior.
- T005 (build + test) is the final gate and depends on T001–T004 all being complete.

<!-- Migrated to plan.md on 2026-06-02 — safe to delete. -->
