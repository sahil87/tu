# Tasks: Add Self-Update Command

**Change**: 260307-9mfs-add-update-command
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Core Implementation

- [x] T001 Add `runUpdate()` function in `src/node/core/cli.ts` — Homebrew detection via `_pkgDir.includes("/Cellar/tu/")`, non-brew message, brew update flow (`brew update --quiet`, `brew info --json=v2 tu`, version comparison, `brew upgrade tu`), error handling with specific messages per spec
- [x] T002 Wire `tu update` dispatch in `main()` in `src/node/core/cli.ts` — add `if (cmd === "update") { runUpdate(); return; }` after the `status` command check

## Phase 2: Help & Tests

- [x] T003 [P] Add `tu update` line to `FULL_HELP` Setup section in `src/node/core/cli.ts` — `tu update            Update tu to latest version`
- [x] T004 [P] Add `assert.ok(FULL_HELP.includes("tu update"))` to the Setup section test in `src/node/core/__tests__/cli-help.test.ts`

---

## Execution Order

- T001 blocks T002 (function must exist before dispatch)
- T003 and T004 are independent of T001-T002 (static string changes), can run in parallel
