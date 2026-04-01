# Tasks: Add -v Version Shorthand

**Change**: 260401-kuuh-add-v-version-shorthand
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Core Implementation

- [x] T001 Add `-v` to version flag condition in `src/node/core/cli.ts` line 1014 — append `|| rawArgs.includes("-v")` to the existing `--version` / `-V` check

---

## Execution Order

No dependencies — single task.
