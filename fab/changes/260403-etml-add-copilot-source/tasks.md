# Tasks: Add Copilot Source

**Change**: 260403-etml-add-copilot-source
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

- [x] T001 Add `@ccusage/copilot` dependency to `package.json` (matching `^18.0.8` version pattern of existing ccusage packages) and run `npm install`

## Phase 2: Core Implementation

- [x] T002 [P] Add `cp` entry to `TOOLS` registry in `src/node/core/fetcher.ts` with `name: "Copilot"`, vendor-aware command path to `ccusage-copilot`, and `needsFilter: true`
- [x] T003 [P] Add `"cp"` to `KNOWN_SOURCES` set and update `FULL_HELP` sources line in `src/node/core/cli.ts`

## Phase 3: Integration & Edge Cases

- [x] T004 [P] Update `src/node/core/__tests__/fetcher.test.ts`: add `TOOLS.cp` existence assertion in "TOOLS" describe block, add `needsFilter` assertion for cp
- [x] T005 [P] Update `src/node/core/__tests__/cli-parser.test.ts`: add test case for `parseDataArgs(["cp"])` returning `{ source: "cp", period: "daily", display: "snapshot" }`

---

## Execution Order

- T001 blocks T002 (dependency must be installed before command path is valid)
- T002 and T003 are independent (different files)
- T004 and T005 are independent (different test files)
