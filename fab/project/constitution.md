# tu Constitution

## Core Principles

### I. Single-Purpose CLI
The tool SHALL remain a focused cost-tracking CLI for AI coding assistants. Feature additions MUST serve the core use case of viewing, aggregating, and syncing usage/cost data. Unrelated functionality (e.g., billing management, AI orchestration) MUST NOT be added.

### II. Graceful Degradation
External dependencies (ccusage binaries, metrics repos, network) MUST NOT crash the CLI. When a data source is unavailable, the tool SHALL warn on stderr and fall back to the best available data (cached, local-only, or zero). The user SHOULD always get *some* output.

### III. Single-Bundle Distribution
The CLI MUST compile to a single ESM bundle via esbuild (`dist/tu.mjs`). Runtime dependencies are bundled — no `node_modules` required at install time. This constraint ensures Homebrew distribution stays simple.

### IV. Fast Startup
The CLI SHOULD minimize startup latency. Heavy operations (network fetches, JSONL scanning) MUST be cached with a reasonable TTL. Imports SHOULD be static (no dynamic `import()` for core paths).

### V. Consistent Data Model
All data flows through `UsageEntry` and `UsageTotals` interfaces. New data sources MUST conform to these types. Aggregation (daily-to-monthly, merge) MUST be pure functions operating on these types. The `label` field MUST use ISO date format (`YYYY-MM-DD` or `YYYY-MM`).

## TypeScript Conventions

- Strict mode is enabled and MUST remain enabled (`"strict": true` in tsconfig)
- Target ES2022 with NodeNext module resolution — MUST use `.js` extensions in imports
- Prefer `node:` prefixed built-in imports (e.g., `node:fs`, `node:path`)
- Use `type` imports for type-only values (`import type { ... }`)
- No classes in the current codebase — prefer functions and plain objects. New code SHOULD follow this pattern unless a class is genuinely warranted

## Additional Constraints

### Test Integrity
Tests MUST conform to the implementation spec — never the other way around. When tests fail, the fix SHALL either (a) update the tests to match the spec, or (b) update the implementation to match the spec. Modifying implementation code solely to accommodate test fixtures or test infrastructure is prohibited. Specs are the source of truth; tests verify conformance to specs.

### Test Runner
Tests use Node.js built-in test runner via `npx tsx --test tests/*.test.ts`. New test files MUST follow the `tests/{module}.test.ts` naming convention. No additional test frameworks SHOULD be introduced without justification.

### Test Location
Node/TypeScript test files MUST be co-located with the source code they test, in `__tests__/` folders within the same directory (e.g., `src/__tests__/fetcher.test.ts` for `src/fetcher.ts`).

### Output Stability
CLI output format (table layouts, color usage, JSON structure) SHOULD remain stable across patch versions. Breaking output changes MUST be accompanied by a minor version bump since downstream scripts may parse the output.

## Governance

**Version**: 1.0.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-03-06
