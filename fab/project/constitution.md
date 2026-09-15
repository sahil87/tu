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
For `src/node/`: tests use the Node.js built-in test runner via `npm test` (`find src/node -path '*/__tests__/*.test.ts' -exec npx tsx --test {} +`). New test files MUST follow the `{module}.test.ts` naming convention inside a `__tests__/` folder (see Test Location). No additional test frameworks SHOULD be introduced without justification. Go tests are governed by the Go Transition article.

### Test Location
For `src/node/`: test files MUST be co-located with the source code they test, in `__tests__/` folders within the same directory (e.g., `src/node/core/__tests__/fetcher.test.ts` for `src/node/core/fetcher.ts`). This clause does not apply to `src/go/` — see the Go Transition article.

### Output Stability
CLI output format (table layouts, color usage, JSON structure) SHOULD remain stable across patch versions. Breaking output changes MUST be accompanied by a minor version bump since downstream scripts may parse the output.

### Toolkit Standards

This tool is part of the shll toolkit and MUST conform to the toolkit's published standards. The standards are enumerated by running `shll standards` — each entry names what it governs; read one with `shll standards <name>`. Before changing the CLI surface, help output, README.md, or docs/site/, the change MUST be checked against the standards governing that surface. If shll is unavailable, the canonical sources are the sahil87/shll repository's docs/site/standards/ tree (rendered on https://shll.ai). Standards added or revised there bind this repo without further amendment to this constitution.

## Go Transition

> Transitional article, added in v1.2.0 (2026-09-15). It governs the period in which `src/node/` and `src/go/` coexist in `main`. It is removed at cutover (v2.0.0), when Principles III and IV, the TypeScript Conventions, and the Test Runner / Test Location constraints are rewritten for the Go implementation. Plan: `fab/plans/sahil/26-09-15-go-port.md` (decisions D2, D3).

- `src/node/` is the **shipped** implementation. `dist/tu.mjs`, the Homebrew formula, and every release artifact are built from it until cutover.
- `src/go/` is the **successor** implementation. It lands in `main` change by change and is built and tested in CI, but it MUST NOT be shipped — not through the formula, not as the `tu` binary users install — until the cutover change flips the formula.
- Principles I (Single-Purpose CLI), II (Graceful Degradation), and V (Consistent Data Model) are language-neutral and bind both trees.
- Principles III (Single-Bundle Distribution) and IV (Fast Startup) and the TypeScript Conventions describe the shipped artifact and bind `src/node/` only. Code under `src/go/` is not in violation of Principle III by existing outside the ESM bundle.
- The Test Runner and Test Location constraints bind `src/node/` only. Go tests MUST be `_test.go` siblings of the code they test, in the same directory (Go idiom). `__tests__/` folders MUST NOT be created under `src/go/`.
- The external surfaces frozen by the plan's Goal — the CLI grammar, `--help` text, table/JSON/CSV/Markdown output, exit codes, `tu.conf` and `org.conf`, the metrics-repo JSONL layout and its never-shrink guard, and the toolkit contracts — are the contract `src/go/` reproduces. Output Stability and Toolkit Standards apply to both trees.

## Governance

**Version**: 1.2.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-15
